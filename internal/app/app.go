package app

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const sessionSeconds = 8 * 60 * 60

type Server struct {
	db   *DB
	root string
	mux  *http.ServeMux
}
type User struct {
	ID, CompanyID                                              int
	Username, DisplayName, Role, PasswordHash, CSRF, TokenHash string
	Active                                                     int
}
type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string       { return e.Message }
func fail(status int, msg string) error { return &apiError{status, msg} }

type resp struct {
	Code    int    `json:"code"`
	Data    any    `json:"data,omitempty"`
	Message string `json:"message"`
	Errors  any    `json:"errors,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp{0, data, "Success", nil})
}
func writeErr(w http.ResponseWriter, err error) {
	ae := &apiError{500, "Internal server error"}
	if errors.As(err, &ae) {
	} else {
		log.Printf("error: %v", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(ae.Status)
	_ = json.NewEncoder(w).Encode(resp{ae.Status, nil, ae.Message, nil})
}

func Run(addr, databaseURL string, demo bool) error {
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	db, err := openPostgres(databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err = db.PingContext(context.Background()); err != nil {
		return fmt.Errorf("database ping: %w", err)
	}
	if err = initSchema(db); err != nil {
		return err
	}
	if demo {
		if err = seedDemo(db); err != nil {
			return err
		}
	}
	root, _ := os.Getwd()
	s := &Server{db: db, root: root, mux: http.NewServeMux()}
	s.routes()
	log.Printf("Go operations admin listening on http://%s", addr)
	return http.ListenAndServe(addr, s.security(s.mux))
}

func (s *Server) security(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			if r.ContentLength > 1<<20 {
				writeErr(w, fail(413, "Request exceeds 1 MiB"))
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				base := "http://" + r.Host
				if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
					base = "https://" + r.Host
				}
				publicOrigin := strings.TrimRight(os.Getenv("PUBLIC_ORIGIN"), "/")
				if origin != base && (publicOrigin == "" || origin != publicOrigin) {
					writeErr(w, fail(403, "Cross-origin write rejected"))
					return
				}
			}
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/web/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.wrap(false, nil, s.health))
	s.mux.HandleFunc("POST /api/auth/login", s.login)
	s.mux.HandleFunc("POST /web/user/login", s.loginLegacy)
	s.mux.HandleFunc("GET /api/auth/me", s.wrap(true, nil, s.me))
	s.mux.HandleFunc("POST /api/auth/logout", s.wrap(true, nil, s.logout))
	s.mux.HandleFunc("POST /web/user/logout", s.wrap(true, nil, s.logout))
	s.mux.HandleFunc("POST /api/auth/password", s.wrap(true, nil, s.changePassword))
	s.mux.HandleFunc("GET /api/users", s.wrap(true, []string{"admin"}, s.users))
	s.mux.HandleFunc("POST /api/users", s.wrap(true, []string{"admin"}, s.addUser))
	s.mux.HandleFunc("POST /api/users/{id}/disable", s.wrap(true, []string{"admin"}, s.disableUser))
	s.mux.HandleFunc("GET /api/meta", s.wrap(true, nil, s.meta))
	s.mux.HandleFunc("GET /api/dashboard", s.wrap(true, nil, s.dashboard))
	s.mux.HandleFunc("GET /api/monitor", s.wrap(true, nil, s.monitor))
	s.mux.HandleFunc("GET /api/earnings", s.wrap(true, nil, s.earnings))
	s.mux.HandleFunc("POST /api/earnings", s.wrap(true, []string{"admin", "operator"}, s.addEarning))
	s.mux.HandleFunc("POST /api/earnings/{id}/adjust", s.wrap(true, []string{"admin", "operator"}, s.adjustEarning))
	s.mux.HandleFunc("GET /api/withdrawals", s.wrap(true, nil, s.withdrawals))
	s.mux.HandleFunc("POST /api/withdrawals", s.wrap(true, []string{"admin", "operator", "finance"}, s.addWithdrawal))
	s.mux.HandleFunc("POST /api/withdrawals/{id}/transition", s.wrap(true, []string{"admin", "finance"}, s.transitionWithdrawal))
	s.mux.HandleFunc("GET /api/settlements", s.wrap(true, nil, s.settlements))
	s.mux.HandleFunc("POST /api/settlements", s.wrap(true, []string{"admin", "finance"}, s.addSettlement))
	s.mux.HandleFunc("POST /api/settlements/{id}/pay", s.wrap(true, []string{"admin", "finance"}, s.paySettlement))
	s.mux.HandleFunc("GET /api/reconciliation", s.wrap(true, nil, s.reconciliation))
	s.mux.HandleFunc("GET /api/reports/{kind}", s.wrap(true, nil, s.report))
	s.mux.HandleFunc("GET /api/audit-logs", s.wrap(true, []string{"admin", "finance"}, s.auditLogs))
	s.mux.HandleFunc("POST /api/accounts/{id}/ban", s.wrap(true, []string{"admin", "operator"}, s.banAccount))
	s.mux.HandleFunc("POST /api/accounts/{id}/unban", s.wrap(true, []string{"admin", "operator"}, s.unbanAccount))
	s.mux.HandleFunc("POST /api/ingest/heartbeat", s.wrap(true, []string{"admin", "operator"}, s.heartbeat))
	s.mux.HandleFunc("GET /api/exports/{resource}", s.wrap(true, []string{"admin", "finance"}, s.exportCSV))
	for _, r := range []string{"channels", "apps", "employees", "teams", "holder-channels", "holders", "accounts", "blacklist", "configs"} {
		s.registerEntity(r)
	}
	s.mux.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir(filepath.Join(s.root, "web", "assets")))))
	s.mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(s.root, "web", "index.html"))
	})
	s.mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
}

type handler func(http.ResponseWriter, *http.Request, *User) error

func (s *Server) wrap(auth bool, roles []string, h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var u *User
		var err error
		if auth {
			u, err = s.currentUser(r)
			if err != nil {
				writeErr(w, err)
				return
			}
			if len(roles) > 0 && !contains(roles, u.Role) {
				writeErr(w, fail(403, "Your role is not allowed to perform this action"))
				return
			}
		}
		if err = h(w, r, u); err != nil {
			writeErr(w, err)
		}
	}
}
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
func decode(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fail(422, "Please check the submitted fields")
	}
	return nil
}
func pathID(r *http.Request) (int, error) {
	i, e := strconv.Atoi(r.PathValue("id"))
	if e != nil || i <= 0 {
		return 0, fail(422, "Invalid id")
	}
	return i, nil
}
func intQ(v url.Values, k string, def int) int {
	n, e := strconv.Atoi(v.Get(k))
	if e != nil || n < 1 {
		return def
	}
	return n
}
func page(v url.Values) (int, int) {
	p := intQ(v, "page", 1)
	sz := intQ(v, "pageSize", 20)
	if sz > 200 {
		sz = 200
	}
	return p, sz
}
func money(c int64) float64  { return float64(c) / 100 }
func cents(v float64) int64  { return int64(math.Round(v * 100)) }
func randToken(n int) string { b := make([]byte, n); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func digest(v string) string { h := sha256.Sum256([]byte(v)); return hex.EncodeToString(h[:]) }
func nullableInt(v any) any {
	if v == nil {
		return nil
	}
	return v
}

func (s *Server) health(w http.ResponseWriter, r *http.Request, u *User) error {
	if err := s.db.Ping(); err != nil {
		return fail(503, "Storage is busy or unavailable; retry shortly")
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "version": "0.2.0-go", "database": "connected"})
	return nil
}

func pbkdf2SHA256(password, salt []byte, iterations, keyLen int) []byte {
	var out []byte
	for block := 1; len(out) < keyLen; block++ {
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(block >> 24), byte(block >> 16), byte(block >> 8), byte(block)})
		u := mac.Sum(nil)
		t := append([]byte(nil), u...)
		for i := 1; i < iterations; i++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		out = append(out, t...)
	}
	return out[:keyLen]
}
func passwordHash(password string) (string, error) {
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", err
	}
	const iters = 200000
	h := pbkdf2SHA256([]byte(password), saltBytes, iters, 32)
	return fmt.Sprintf("pbkdf2_sha256$%d$%s$%s", iters, hex.EncodeToString(saltBytes), hex.EncodeToString(h)), nil
}
func verifyPassword(password, encoded string) bool {
	p := strings.Split(encoded, "$")
	if len(p) != 4 || p[0] != "pbkdf2_sha256" {
		return false
	}
	iters, err := strconv.Atoi(p[1])
	if err != nil || iters < 10000 {
		return false
	}
	salt, err := hex.DecodeString(p[2])
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(p[3])
	if err != nil {
		return false
	}
	actual := pbkdf2SHA256([]byte(password), salt, iters, len(expected))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func (s *Server) currentUser(r *http.Request) (*User, error) {
	token := r.Header.Get("X-Token")
	header := token != ""
	if token == "" && strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		token = strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		header = true
	}
	if token == "" {
		if c, e := r.Cookie("ops_session"); e == nil {
			token = c.Value
		}
	}
	if token == "" {
		return nil, fail(401, "Please sign in")
	}
	u := &User{}
	err := s.db.QueryRow(`SELECT u.id,u.company_id,u.username,u.display_name,u.role,u.password_hash,u.active,s.csrf,s.token_hash FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token_hash=? AND s.expires_at>? AND u.active=1`, digest(token), time.Now().Unix()).Scan(&u.ID, &u.CompanyID, &u.Username, &u.DisplayName, &u.Role, &u.PasswordHash, &u.Active, &u.CSRF, &u.TokenHash)
	if err == sql.ErrNoRows {
		return nil, fail(401, "Session expired or revoked")
	}
	if err != nil {
		return nil, err
	}
	if !header && r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(u.CSRF)) != 1 {
		return nil, fail(403, "Invalid CSRF token")
	}
	return u, nil
}
func (s *Server) userInfo(u *User) map[string]any {
	var company string
	_ = s.db.QueryRow("SELECT name FROM companies WHERE id=?", u.CompanyID).Scan(&company)
	return map[string]any{"id": u.ID, "name": u.DisplayName, "username": u.Username, "role": u.Role, "roles": []string{u.Role}, "companyId": u.CompanyID, "companyName": company, "csrfToken": u.CSRF}
}
func (s *Server) audit(u *User, action, entity string, id any, detail any) {
	b, _ := json.Marshal(detail)
	_, _ = s.db.Exec("INSERT INTO audit_logs(company_id,user_id,action,entity,entity_id,detail) VALUES (?,?,?,?,?,?)", u.CompanyID, u.ID, action, entity, id, string(b))
}

func (s *Server) loginCore(r *http.Request) (string, *User, error) {
	var p struct{ Username, Password string }
	if e := decode(r, &p); e != nil {
		return "", nil, e
	}
	if strings.TrimSpace(p.Username) == "" || p.Password == "" {
		return "", nil, fail(422, "Please check the submitted fields")
	}
	now := time.Now().Unix()
	ip := "unknown"
	if h, _, e := net.SplitHostPort(r.RemoteAddr); e == nil {
		ip = h
	}
	buckets := []string{digest("user:" + strings.ToLower(p.Username)), digest("ip:" + ip)}
	_, _ = s.db.Exec("DELETE FROM login_attempts WHERE created_at<?", now-900)
	for i, b := range buckets {
		max := 8
		if i == 1 {
			max = 40
		}
		var n int
		_ = s.db.QueryRow("SELECT count(*) FROM login_attempts WHERE bucket=? AND created_at>?", b, now-900).Scan(&n)
		if n >= max {
			return "", nil, fail(429, "Too many login attempts; try again in 15 minutes")
		}
	}
	u := &User{}
	err := s.db.QueryRow("SELECT id,company_id,username,display_name,role,password_hash,active FROM users WHERE username=?", p.Username).Scan(&u.ID, &u.CompanyID, &u.Username, &u.DisplayName, &u.Role, &u.PasswordHash, &u.Active)
	valid := err == nil && u.Active == 1 && verifyPassword(p.Password, u.PasswordHash)
	if !valid {
		for _, b := range buckets {
			_, _ = s.db.Exec("INSERT INTO login_attempts(bucket,created_at) VALUES (?,?)", b, now)
		}
		return "", nil, fail(401, "Incorrect username or password")
	}
	_, _ = s.db.Exec("DELETE FROM login_attempts WHERE bucket=?", buckets[0])
	_, _ = s.db.Exec("DELETE FROM sessions WHERE expires_at<=?", now)
	token := randToken(40)
	u.CSRF = randToken(32)
	u.TokenHash = digest(token)
	_, err = s.db.Exec("INSERT INTO sessions(token_hash,user_id,csrf,expires_at) VALUES (?,?,?,?)", u.TokenHash, u.ID, u.CSRF, now+sessionSeconds)
	if err != nil {
		return "", nil, err
	}
	s.audit(u, "login", "users", u.ID, map[string]any{})
	return token, u, nil
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	token, u, e := s.loginCore(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "ops_session", Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: sessionSeconds, Secure: os.Getenv("OPS_SECURE_COOKIE") == "1"})
	writeJSON(w, 200, s.userInfo(u))
}
func (s *Server) loginLegacy(w http.ResponseWriter, r *http.Request) {
	token, u, e := s.loginCore(r)
	if e != nil {
		writeErr(w, e)
		return
	}
	d := s.userInfo(u)
	d["token"] = token
	writeJSON(w, 200, d)
}
func (s *Server) me(w http.ResponseWriter, r *http.Request, u *User) error {
	writeJSON(w, 200, s.userInfo(u))
	return nil
}
func (s *Server) logout(w http.ResponseWriter, r *http.Request, u *User) error {
	_, _ = s.db.Exec("DELETE FROM sessions WHERE token_hash=?", u.TokenHash)
	s.audit(u, "logout", "users", u.ID, map[string]any{})
	http.SetCookie(w, &http.Cookie{Name: "ops_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{})
	return nil
}
func (s *Server) changePassword(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct{ OldPassword, NewPassword string }
	if e := decode(r, &p); e != nil {
		return e
	}
	if !verifyPassword(p.OldPassword, u.PasswordHash) {
		return fail(403, "Current password is incorrect")
	}
	if len(p.NewPassword) < 12 {
		return fail(422, "The new password must be at least 12 characters")
	}
	if p.OldPassword == p.NewPassword {
		return fail(422, "The new password must be different")
	}
	h, e := passwordHash(p.NewPassword)
	if e != nil {
		return e
	}
	tx, _ := s.db.Begin()
	defer tx.Rollback()
	if _, e = tx.Exec("UPDATE users SET password_hash=? WHERE id=?", h, u.ID); e != nil {
		return e
	}
	if _, e = tx.Exec("DELETE FROM sessions WHERE user_id=?", u.ID); e != nil {
		return e
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	http.SetCookie(w, &http.Cookie{Name: "ops_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]any{"signInAgain": true})
	return nil
}

func (s *Server) users(w http.ResponseWriter, r *http.Request, u *User) error {
	rows, e := s.db.Query("SELECT id,username,display_name,role,active,created_at FROM users WHERE company_id=? ORDER BY id", u.CompanyID)
	if e != nil {
		return e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, active int
		var a, b, c, d string
		if e := rows.Scan(&id, &a, &b, &c, &active, &d); e != nil {
			return e
		}
		list = append(list, map[string]any{"id": id, "username": a, "displayName": b, "role": c, "active": active, "createDate": d})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": len(list)})
	return nil
}
func (s *Server) addUser(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct{ Username, DisplayName, Password, Role string }
	if e := decode(r, &p); e != nil {
		return e
	}
	if len(p.Username) < 3 || len(p.Password) < 12 || !contains([]string{"admin", "operator", "finance", "viewer"}, p.Role) {
		return fail(422, "Please check the submitted fields")
	}
	h, e := passwordHash(p.Password)
	if e != nil {
		return e
	}
	res, e := s.db.Exec("INSERT INTO users(company_id,username,display_name,password_hash,role) VALUES (?,?,?,?,?)", u.CompanyID, p.Username, p.DisplayName, h, p.Role)
	if e != nil {
		return dbConflict(e)
	}
	id, _ := res.LastInsertId()
	s.audit(u, "create", "users", id, map[string]any{"role": p.Role})
	writeJSON(w, 200, map[string]any{"id": id})
	return nil
}
func (s *Server) disableUser(w http.ResponseWriter, r *http.Request, u *User) error {
	id, e := pathID(r)
	if e != nil {
		return e
	}
	if id == u.ID {
		return fail(409, "Cannot disable your own account")
	}
	res, e := s.db.Exec("UPDATE users SET active=0 WHERE id=? AND company_id=?", id, u.CompanyID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fail(404, "Record not found in your company")
	}
	_, _ = s.db.Exec("DELETE FROM sessions WHERE user_id=?", id)
	s.audit(u, "disable", "users", id, map[string]any{})
	writeJSON(w, 200, map[string]any{"disabled": true})
	return nil
}

// Remaining business and entity handlers are implemented below in the same package.
func initSchema(db *DB) error {
	b, err := os.ReadFile("backend/schema.sql")
	if err != nil {
		return fmt.Errorf("read schema: %w", err)
	}
	for _, stmt := range strings.Split(string(b), ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err = db.Exec(stmt); err != nil {
			return fmt.Errorf("schema statement failed: %w", err)
		}
	}
	return nil
}

func seedDemo(db *DB) error {
	var n int
	if err := db.QueryRow("SELECT count(*) FROM companies").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT INTO settings(key,value) VALUES ('demo','true')"); err != nil {
		return err
	}
	if _, err = tx.Exec("INSERT INTO companies(id,name) VALUES (1,?)", "演示运营公司"); err != nil {
		return err
	}
	users := []struct{ u, p, r, n string }{{"admin", "DemoAdmin2026!", "admin", "系统管理员"}, {"operator", "DemoOperator2026!", "operator", "运营专员"}, {"finance", "DemoFinance2026!", "finance", "财务审核员"}, {"viewer", "DemoViewer2026!", "viewer", "只读访客"}}
	for i, x := range users {
		h, e := passwordHash(x.p)
		if e != nil {
			return e
		}
		if _, e = tx.Exec("INSERT INTO users(id,company_id,username,display_name,password_hash,role) VALUES (?,1,?,?,?,?)", i+1, x.u, x.n, h, x.r); e != nil {
			return e
		}
	}
	for i, nm := range []string{"华东运营中心", "华南运营中心"} {
		if _, err = tx.Exec("INSERT INTO channels(id,company_id,name) VALUES (?,1,?)", i+1, nm); err != nil {
			return err
		}
	}
	for i, nm := range []string{"平台 A", "平台 B", "平台 C", "平台 D"} {
		if _, err = tx.Exec("INSERT INTO apps(id,company_id,name,package_name) VALUES (?,1,?,?)", i+1, nm, fmt.Sprintf("demo.platform.%d", i+1)); err != nil {
			return err
		}
	}
	teams := []struct {
		name string
		ch   int
	}{{"星河一组", 1}, {"远山二组", 1}, {"南风小组", 2}}
	for i, t := range teams {
		if _, err = tx.Exec("INSERT INTO teams(id,company_id,channel_id,name) VALUES (?,1,?,?)", i+1, t.ch, t.name); err != nil {
			return err
		}
	}
	for i := 1; i <= 6; i++ {
		team := (i-1)/2 + 1
		ch := 1
		if team >= 3 {
			ch = 2
		}
		if _, err = tx.Exec("INSERT INTO employees(id,company_id,channel_id,team_id,name) VALUES (?,1,?,?,?)", i, ch, team, fmt.Sprintf("演示员工 %c", 64+i)); err != nil {
			return err
		}
	}
	for _, x := range [][2]int{{1, 1}, {2, 3}, {3, 5}} {
		_, _ = tx.Exec("UPDATE teams SET leader_id=? WHERE id=?", x[1], x[0])
	}
	for i := 1; i <= 2; i++ {
		_, _ = tx.Exec("INSERT INTO holder_channels(id,company_id,channel_id,name,percent_bps) VALUES (?,1,?,?,500)", i, i, fmt.Sprintf("演示实名渠道 %d", i))
	}
	for i := 1; i <= 4; i++ {
		ch := 1
		if i >= 3 {
			ch = 2
		}
		_, _ = tx.Exec("INSERT INTO holders(id,company_id,channel_id,holder_channel_id,name,bak_name,percent_bps) VALUES (?,1,?,?,?,?,?)", i, ch, ch, fmt.Sprintf("实名演示人员 %d", i), fmt.Sprintf("H%d", i), 1000+i*100)
	}
	now := time.Now().UTC()
	today := time.Now().Format("2006-01-02")
	for i := 1; i <= 20; i++ {
		emp := (i-1)%6 + 1
		team := (emp-1)/2 + 1
		ch := 1
		if emp >= 5 {
			ch = 2
		}
		holder := 1 + (i % 2)
		if ch == 2 {
			holder = 3 + (i % 2)
		}
		var hid any = holder
		if i == 8 || i == 11 {
			hid = nil
		}
		st := 0
		if i == 19 {
			st = 1
		} else if i == 20 {
			st = 2
		}
		last := now.Add(-time.Duration(i%5) * time.Hour).Format("2006-01-02 15:04:05")
		created := now.AddDate(0, 0, -30-i).Format("2006-01-02 15:04:05")
		_, err = tx.Exec(`INSERT INTO accounts(id,company_id,channel_id,app_id,employee_id,holder_id,uid,name,bak_name,status,opening_cents,gift_account,last_seen,created_at) VALUES (?,1,?,?,?,?,?,?,?,?,?,?,?,?)`, i, ch, (i-1)%4+1, emp, hid, fmt.Sprintf("DEMO-%d", 10000+i), fmt.Sprintf("演示账号 %02d", i), fmt.Sprintf("A%02d", i), st, 30000+i*127, map[bool]int{true: 1, false: 0}[i == 5 || i == 12 || i == 17], last, created)
		if err != nil {
			return err
		}
		_, _ = tx.Exec("INSERT INTO configs(company_id,account_id,friends_circle_end_date,close_friends_end_date,note) VALUES (1,?,?,?,?)", i, time.Now().AddDate(0, 0, i-10).Format("2006-01-02"), time.Now().AddDate(0, 0, 15).Format("2006-01-02"), "仅演示配置登记")
		for back := 0; back < 14; back++ {
			if back == 0 && (i == 6 || i == 9 || i == 14 || i == 19 || i == 20) {
				continue
			}
			amount := 1700 + ((i*271 + back*431) % 4800)
			day := time.Now().AddDate(0, 0, -back).Format("2006-01-02")
			_, err = tx.Exec(`INSERT INTO earnings(company_id,account_id,employee_id,team_id,channel_id,day,hour,amount_cents,kind,source_ref,note,created_by) VALUES (1,?,?,?,?,?,?,?,'income',?,?,2)`, i, emp, team, ch, day, (i+8)%24, amount, fmt.Sprintf("DEMO-INCOME-%d-%s", i, day), "合成演示数据")
			if err != nil {
				return err
			}
		}
	}
	defs := []struct {
		account int
		state   string
		back    int
	}{{1, "paid", 3}, {2, "paid", 2}, {5, "paid", 1}, {7, "paid", 0}, {3, "pending", 0}, {4, "pending", 0}, {6, "approved", 0}, {10, "rejected", 1}}
	for i, d := range defs {
		var holder, hc, percent int
		_ = tx.QueryRow("SELECT holder_id FROM accounts WHERE id=?", d.account).Scan(&holder)
		_ = tx.QueryRow("SELECT holder_channel_id,percent_bps FROM holders WHERE id=?", holder).Scan(&hc, &percent)
		amount := 15000 + (i+1)*1000
		share := int(math.Round(float64(amount*percent) / 10000))
		moment := now.AddDate(0, 0, -d.back).Add(-time.Hour).Format("2006-01-02 15:04:05")
		var reviewer, pref, paid any
		if d.state != "pending" {
			reviewer = 3
		}
		if d.state == "paid" {
			pref = fmt.Sprintf("DEMO-REFERENCE-%d", i+1)
			paid = moment
		}
		_, err = tx.Exec(`INSERT INTO withdrawals(id,company_id,account_id,holder_id,holder_channel_id,amount_cents,percent_bps,holder_share_cents,state,idempotency_key,payload_hash,note,created_by,reviewed_by,payment_ref,paid_at,created_at) VALUES (?,1,?,?,?,?,?,?,?,?,?,?,2,?,?,?,?)`, i+1, d.account, holder, hc, amount, percent, share, d.state, fmt.Sprintf("DEMO-WITHDRAW-%d", i+1), "demo-seeded", "演示账务，无真实打款", reviewer, pref, paid, moment)
		if err != nil {
			return err
		}
	}
	_, _ = tx.Exec("INSERT INTO bans(company_id,account_id,employee_id,reason,balance_cents,created_by) VALUES (1,19,1,?,45000,2)", "演示封禁记录，未确认实际损失")
	_, _ = tx.Exec("INSERT INTO blacklist(company_id,app_id,uid,reason) VALUES (1,1,?,?)", "DEMO-BLOCKED-001", "演示黑名单记录")
	_, _ = tx.Exec("INSERT INTO audit_logs(company_id,user_id,action,entity,detail) VALUES (1,1,'seed_demo','system','{}')")
	for _, table := range []string{"companies", "users", "channels", "apps", "teams", "employees", "holder_channels", "holders", "accounts", "earnings", "withdrawals", "blacklist", "configs", "bans", "audit_logs"} {
		var ignored int64
		q := fmt.Sprintf("SELECT setval(pg_get_serial_sequence('%s','id'), GREATEST(COALESCE(MAX(id),1),1), true) FROM %s", table, table)
		if err = tx.QueryRow(q).Scan(&ignored); err != nil {
			return err
		}
	}
	_ = today
	return tx.Commit()
}

func dbConflict(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "constraint") {
		return fail(409, "A duplicate value or a linked record prevents this operation")
	}
	return err
}

func (s *Server) registerEntity(resource string) {
	s.mux.HandleFunc("GET /api/"+resource, s.wrap(true, nil, func(w http.ResponseWriter, r *http.Request, u *User) error {
		d, e := s.entityList(resource, u, r.URL.Query(), false)
		if e != nil {
			return e
		}
		writeJSON(w, 200, d)
		return nil
	}))
	s.mux.HandleFunc("GET /api/"+resource+"/{id}", s.wrap(true, nil, func(w http.ResponseWriter, r *http.Request, u *User) error {
		id, e := pathID(r)
		if e != nil {
			return e
		}
		m, e := s.entityGet(resource, id, u)
		if e != nil {
			return e
		}
		writeJSON(w, 200, m)
		return nil
	}))
	s.mux.HandleFunc("POST /api/"+resource, s.wrap(true, []string{"admin", "operator"}, func(w http.ResponseWriter, r *http.Request, u *User) error {
		m, e := decodeMap(r)
		if e != nil {
			return e
		}
		id, e := s.entitySave(resource, 0, m, u)
		if e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"id": id})
		return nil
	}))
	s.mux.HandleFunc("PUT /api/"+resource+"/{id}", s.wrap(true, []string{"admin", "operator"}, func(w http.ResponseWriter, r *http.Request, u *User) error {
		id, e := pathID(r)
		if e != nil {
			return e
		}
		m, e := decodeMap(r)
		if e != nil {
			return e
		}
		_, e = s.entitySave(resource, id, m, u)
		if e != nil {
			return e
		}
		out, e := s.entityGet(resource, id, u)
		if e != nil {
			return e
		}
		writeJSON(w, 200, out)
		return nil
	}))
	s.mux.HandleFunc("DELETE /api/"+resource+"/{id}", s.wrap(true, []string{"admin"}, func(w http.ResponseWriter, r *http.Request, u *User) error {
		id, e := pathID(r)
		if e != nil {
			return e
		}
		e = s.entityDelete(resource, id, intQ(r.URL.Query(), "version", 0), u)
		if e != nil {
			return e
		}
		writeJSON(w, 200, map[string]any{"deleted": true})
		return nil
	}))
}
func decodeMap(r *http.Request) (map[string]any, error) {
	var m map[string]any
	if e := decode(r, &m); e != nil {
		return nil, e
	}
	return m, nil
}

type entityDef struct{ table string }

var entityDefs = map[string]entityDef{"channels": {"channels"}, "apps": {"apps"}, "employees": {"employees"}, "teams": {"teams"}, "holder-channels": {"holder_channels"}, "holders": {"holders"}, "accounts": {"accounts"}, "blacklist": {"blacklist"}, "configs": {"configs"}}

func (s *Server) entityList(resource string, u *User, q url.Values, all bool) (map[string]any, error) {
	def, ok := entityDefs[resource]
	if !ok {
		return nil, fail(404, "Unknown resource")
	}
	where := []string{"t.company_id=?"}
	args := []any{u.CompanyID}
	if v := q.Get("status"); v != "" {
		where = append(where, "t.status=?")
		args = append(args, v)
	}
	if v := q.Get("channelId"); v != "" && resource != "channels" && resource != "apps" && resource != "blacklist" && resource != "configs" {
		where = append(where, "t.channel_id=?")
		args = append(args, v)
	}
	if v := q.Get("keyword"); v != "" {
		pat := "%" + v + "%"
		switch resource {
		case "accounts":
			where = append(where, "(t.name LIKE ? OR t.uid LIKE ? OR t.bak_name LIKE ?)")
			args = append(args, pat, pat, pat)
		case "blacklist":
			where = append(where, "(t.uid LIKE ? OR t.reason LIKE ?)")
			args = append(args, pat, pat)
		default:
			where = append(where, "t.name LIKE ?")
			args = append(args, pat)
		}
	}
	joins := ""
	cols := "t.*"
	switch resource {
	case "employees":
		joins = " LEFT JOIN channels c ON c.id=t.channel_id LEFT JOIN teams tm ON tm.id=t.team_id"
		cols += ",c.name channelName,tm.name teamName,(SELECT count(*) FROM accounts a WHERE a.employee_id=t.id) accountCount"
	case "teams":
		joins = " LEFT JOIN channels c ON c.id=t.channel_id LEFT JOIN employees e ON e.id=t.leader_id"
		cols += ",c.name channelName,e.name leaderName,(SELECT count(*) FROM employees x WHERE x.team_id=t.id) employeeCount"
	case "holder-channels":
		joins = " LEFT JOIN channels c ON c.id=t.channel_id"
		cols += ",c.name channelName"
	case "holders":
		joins = " LEFT JOIN channels c ON c.id=t.channel_id LEFT JOIN holder_channels hc ON hc.id=t.holder_channel_id"
		cols += ",c.name channelName,hc.name accountChannelName"
	case "accounts":
		joins = " LEFT JOIN channels c ON c.id=t.channel_id LEFT JOIN apps ap ON ap.id=t.app_id LEFT JOIN employees e ON e.id=t.employee_id LEFT JOIN teams tm ON tm.id=e.team_id LEFT JOIN holders h ON h.id=t.holder_id"
		cols += ",c.name channelName,ap.name appName,e.name employeeName,tm.name teamName,h.name holderName"
	case "blacklist":
		joins = " LEFT JOIN apps ap ON ap.id=t.app_id"
		cols += ",ap.name appName,t.uid chatAccountUid"
	case "configs":
		joins = " LEFT JOIN accounts a ON a.id=t.account_id LEFT JOIN apps ap ON ap.id=a.app_id LEFT JOIN employees e ON e.id=a.employee_id"
		cols += ",a.name chatAccountName,a.bak_name chatAccountBakName,ap.name appName,e.name employeeName"
	}
	base := " FROM " + def.table + " t" + joins + " WHERE " + strings.Join(where, " AND ")
	var total int
	if e := s.db.QueryRow("SELECT count(*) FROM "+def.table+" t WHERE "+strings.Join(where, " AND "), args...).Scan(&total); e != nil {
		return nil, e
	}
	p, sz := page(q)
	sqlq := "SELECT " + cols + base + " ORDER BY t.id DESC"
	qargs := append([]any{}, args...)
	if !all {
		sqlq += " LIMIT ? OFFSET ?"
		qargs = append(qargs, sz, (p-1)*sz)
	}
	rows, e := queryMaps(s.db, sqlq, qargs...)
	if e != nil {
		return nil, e
	}
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		m, e := s.publicEntity(resource, row)
		if e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return map[string]any{"list": out, "total": total, "page": p, "pageSize": sz, "truncated": false}, nil
}
func queryMaps(db *DB, q string, args ...any) ([]map[string]any, error) {
	rows, e := db.Query(q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	out := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range vals {
			ptr[i] = &vals[i]
		}
		if e = rows.Scan(ptr...); e != nil {
			return nil, e
		}
		m := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			m[c] = v
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	case string:
		n, _ := strconv.ParseInt(x, 10, 64)
		return n
	}
	return 0
}
func strv(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func camel(k string) string {
	m := map[string]string{"company_id": "companyId", "created_at": "createDate", "channel_id": "channelId", "app_id": "appId", "team_id": "teamId", "leader_id": "leaderId", "employee_id": "employeeId", "holder_id": "accountHolderId", "holder_channel_id": "accountChannelId", "uid": "chatAccountUid", "bak_name": "bakName", "status": "status", "opening_cents": "openingBalance", "gift_account": "giftAccount", "package_name": "packageName", "percent_bps": "percent", "account_valid_days": "accountValidDays", "account_id": "accountId", "friends_circle_end_date": "friendsCircleEndDate", "close_friends_end_date": "closeFriendsEndDate", "last_seen": "lastSeen"}
	if x, ok := m[k]; ok {
		return x
	}
	return k
}
func (s *Server) publicEntity(resource string, row map[string]any) (map[string]any, error) {
	m := map[string]any{}
	for k, v := range row {
		m[camel(k)] = v
	}
	if v, ok := m["openingBalance"]; ok {
		m["openingBalance"] = money(asInt(v))
	}
	if v, ok := m["percent"]; ok {
		m["percent"] = money(asInt(v))
	}
	if resource == "accounts" {
		m["chatAccountName"] = row["name"]
		m["chatAccountUid"] = row["uid"]
		m["chatAccountBakName"] = row["bak_name"]
		m["accountStatus"] = row["status"]
		bal, av, held, e := s.accountBalances(int(asInt(row["id"])), int(asInt(row["company_id"])), asInt(row["opening_cents"]))
		if e != nil {
			return nil, e
		}
		m["total"] = money(bal)
		m["available"] = money(av)
		m["held"] = money(held)
	}
	return m, nil
}
func (s *Server) entityGet(resource string, id int, u *User) (map[string]any, error) {
	q := url.Values{}
	d, e := s.entityList(resource, u, q, true)
	if e != nil {
		return nil, e
	}
	for _, x := range d["list"].([]map[string]any) {
		if int(asInt(x["id"])) == id {
			return x, nil
		}
	}
	return nil, fail(404, "Record not found in your company")
}

func num(m map[string]any, k string) int64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	return asInt(v)
}
func fnum(m map[string]any, k string) float64 {
	v, ok := m[k]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}
func mustName(m map[string]any) (string, error) {
	n := strings.TrimSpace(strv(m["name"]))
	if n == "" || len(n) > 80 {
		return "", fail(422, "Please check the submitted fields")
	}
	return n, nil
}
func (s *Server) entitySave(resource string, id int, m map[string]any, u *User) (int64, error) {
	def := entityDefs[resource]
	if def.table == "" {
		return 0, fail(404, "Unknown resource")
	}
	var e error
	var res sql.Result
	ver := num(m, "version")
	status := num(m, "status")
	switch resource {
	case "channels":
		n, e := mustName(m)
		if e != nil {
			return 0, e
		}
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO channels(company_id,name,status) VALUES (?,?,?)", u.CompanyID, n, status)
		} else {
			res, e = s.db.Exec("UPDATE channels SET name=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", n, status, id, u.CompanyID, ver)
		}
	case "apps":
		n, e2 := mustName(m)
		if e2 != nil {
			return 0, e2
		}
		pkg := strv(m["packageName"])
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO apps(company_id,name,package_name,status) VALUES (?,?,?,?)", u.CompanyID, n, pkg, status)
		} else {
			res, e = s.db.Exec("UPDATE apps SET name=?,package_name=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", n, pkg, status, id, u.CompanyID, ver)
		}
	case "employees":
		n, e2 := mustName(m)
		if e2 != nil {
			return 0, e2
		}
		ch := num(m, "channelId")
		team := num(m, "teamId")
		var tv any
		if team > 0 {
			tv = team
		}
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO employees(company_id,channel_id,team_id,name,status) VALUES (?,?,?,?,?)", u.CompanyID, ch, tv, n, status)
		} else {
			res, e = s.db.Exec("UPDATE employees SET channel_id=?,team_id=?,name=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", ch, tv, n, status, id, u.CompanyID, ver)
		}
	case "teams":
		n, e2 := mustName(m)
		if e2 != nil {
			return 0, e2
		}
		ch := num(m, "channelId")
		leader := num(m, "leaderId")
		var lv any
		if leader > 0 {
			lv = leader
		}
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO teams(company_id,channel_id,name,leader_id,status) VALUES (?,?,?,?,?)", u.CompanyID, ch, n, lv, status)
		} else {
			res, e = s.db.Exec("UPDATE teams SET channel_id=?,name=?,leader_id=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", ch, n, lv, status, id, u.CompanyID, ver)
		}
	case "holder-channels":
		n, e2 := mustName(m)
		if e2 != nil {
			return 0, e2
		}
		ch := num(m, "channelId")
		pct := int64(math.Round(fnum(m, "percent") * 100))
		days := num(m, "accountValidDays")
		if days == 0 {
			days = 30
		}
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO holder_channels(company_id,channel_id,name,percent_bps,account_valid_days,status) VALUES (?,?,?,?,?,?)", u.CompanyID, ch, n, pct, days, status)
		} else {
			res, e = s.db.Exec("UPDATE holder_channels SET channel_id=?,name=?,percent_bps=?,account_valid_days=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", ch, n, pct, days, status, id, u.CompanyID, ver)
		}
	case "holders":
		n, e2 := mustName(m)
		if e2 != nil {
			return 0, e2
		}
		ch := num(m, "channelId")
		hc := num(m, "accountChannelId")
		var hv any
		if hc > 0 {
			hv = hc
		}
		pct := int64(math.Round(fnum(m, "percent") * 100))
		if pct == 0 {
			pct = 1000
		}
		bak := strv(m["bakName"])
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO holders(company_id,channel_id,holder_channel_id,name,bak_name,percent_bps,status) VALUES (?,?,?,?,?,?,?)", u.CompanyID, ch, hv, n, bak, pct, status)
		} else {
			res, e = s.db.Exec("UPDATE holders SET channel_id=?,holder_channel_id=?,name=?,bak_name=?,percent_bps=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", ch, hv, n, bak, pct, status, id, u.CompanyID, ver)
		}
	case "accounts":
		ch, app := num(m, "channelId"), num(m, "appId")
		emp, holder := num(m, "employeeId"), num(m, "accountHolderId")
		var ev, hv any
		if emp > 0 {
			ev = emp
		}
		if holder > 0 {
			hv = holder
		}
		uid, name := strings.TrimSpace(strv(m["chatAccountUid"])), strings.TrimSpace(strv(m["chatAccountName"]))
		if uid == "" || name == "" {
			return 0, fail(422, "Please check the submitted fields")
		}
		bak := strv(m["chatAccountBakName"])
		st := num(m, "accountStatus")
		open := cents(fnum(m, "openingBalance"))
		gift := num(m, "giftAccount")
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO accounts(company_id,channel_id,app_id,employee_id,holder_id,uid,name,bak_name,status,opening_cents,gift_account) VALUES (?,?,?,?,?,?,?,?,?,?,?)", u.CompanyID, ch, app, ev, hv, uid, name, bak, st, open, gift)
		} else {
			res, e = s.db.Exec("UPDATE accounts SET channel_id=?,app_id=?,employee_id=?,holder_id=?,uid=?,name=?,bak_name=?,status=?,opening_cents=?,gift_account=?,version=version+1 WHERE id=? AND company_id=? AND version=?", ch, app, ev, hv, uid, name, bak, st, open, gift, id, u.CompanyID, ver)
		}
	case "blacklist":
		app := num(m, "appId")
		uid, reason := strings.TrimSpace(strv(m["uid"])), strings.TrimSpace(strv(m["reason"]))
		if uid == "" || len(reason) < 3 {
			return 0, fail(422, "Please check the submitted fields")
		}
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO blacklist(company_id,app_id,uid,reason,status) VALUES (?,?,?,?,?)", u.CompanyID, app, uid, reason, status)
		} else {
			res, e = s.db.Exec("UPDATE blacklist SET app_id=?,uid=?,reason=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", app, uid, reason, status, id, u.CompanyID, ver)
		}
	case "configs":
		aid := num(m, "accountId")
		if aid <= 0 {
			return 0, fail(422, "Please check the submitted fields")
		}
		f1, f2, note := strv(m["friendsCircleEndDate"]), strv(m["closeFriendsEndDate"]), strv(m["note"])
		if id == 0 {
			res, e = s.db.Exec("INSERT INTO configs(company_id,account_id,friends_circle_end_date,close_friends_end_date,note,status) VALUES (?,?,?,?,?,?)", u.CompanyID, aid, nullStr(f1), nullStr(f2), note, status)
		} else {
			res, e = s.db.Exec("UPDATE configs SET account_id=?,friends_circle_end_date=?,close_friends_end_date=?,note=?,status=?,version=version+1 WHERE id=? AND company_id=? AND version=?", aid, nullStr(f1), nullStr(f2), note, status, id, u.CompanyID, ver)
		}
	}
	if e != nil {
		return 0, dbConflict(e)
	}
	if id > 0 {
		n, _ := res.RowsAffected()
		if n == 0 {
			return 0, fail(409, "Record changed; refresh before retrying")
		}
		s.audit(u, "update", def.table, id, map[string]any{})
		return int64(id), nil
	}
	newid, _ := res.LastInsertId()
	s.audit(u, "create", def.table, newid, map[string]any{})
	return newid, nil
}
func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
func (s *Server) entityDelete(resource string, id, version int, u *User) error {
	def := entityDefs[resource]
	if def.table == "" {
		return fail(404, "Unknown resource")
	}
	res, e := s.db.Exec("DELETE FROM "+def.table+" WHERE id=? AND company_id=? AND version=?", id, u.CompanyID, version)
	if e != nil {
		return dbConflict(e)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fail(409, "Record changed or not found; refresh before retrying")
	}
	s.audit(u, "delete", def.table, id, map[string]any{})
	return nil
}
func (s *Server) accountBalances(id, company int, opening int64) (int64, int64, int64, error) {
	var inc, paid, held int64
	if e := s.db.QueryRow("SELECT COALESCE(SUM(amount_cents),0) FROM earnings WHERE account_id=? AND company_id=?", id, company).Scan(&inc); e != nil {
		return 0, 0, 0, e
	}
	if e := s.db.QueryRow("SELECT COALESCE(SUM(CASE WHEN state='paid' THEN amount_cents ELSE 0 END),0),COALESCE(SUM(CASE WHEN state IN ('pending','approved') THEN amount_cents ELSE 0 END),0) FROM withdrawals WHERE account_id=? AND company_id=?", id, company).Scan(&paid, &held); e != nil {
		return 0, 0, 0, e
	}
	bal := opening + inc - paid
	return bal, bal - held, held, nil
}

func (s *Server) meta(w http.ResponseWriter, r *http.Request, u *User) error {
	d := map[string]any{}
	for _, x := range []string{"channels", "apps", "employees", "teams", "holders", "holder-channels", "accounts"} {
		m, e := s.entityList(x, u, url.Values{}, true)
		if e != nil {
			return e
		}
		d[x] = m
	}
	d["user"] = s.userInfo(u)
	var n int
	_ = s.db.QueryRow("SELECT count(*) FROM settings WHERE key='demo'").Scan(&n)
	d["demo"] = n > 0
	writeJSON(w, 200, d)
	return nil
}

func dateRange(q url.Values, days int) (string, string, error) {
	end := q.Get("endDate")
	if end == "" {
		end = time.Now().Format("2006-01-02")
	}
	et, e := time.Parse("2006-01-02", end)
	if e != nil {
		return "", "", fail(422, "Dates must use YYYY-MM-DD")
	}
	start := q.Get("startDate")
	if start == "" {
		start = et.AddDate(0, 0, -days+1).Format("2006-01-02")
	}
	st, e := time.Parse("2006-01-02", start)
	if e != nil {
		return "", "", fail(422, "Dates must use YYYY-MM-DD")
	}
	if st.After(et) || et.Sub(st) > 367*24*time.Hour {
		return "", "", fail(422, "Date range must be ordered and no longer than 367 days")
	}
	return start, end, nil
}

func (s *Server) earningsData(u *User, q url.Values, all bool) (map[string]any, error) {
	start, end, e := dateRange(q, 30)
	if e != nil {
		return nil, e
	}
	where := []string{"e.company_id=?", "e.day BETWEEN ? AND ?"}
	args := []any{u.CompanyID, start, end}
	for k, col := range map[string]string{"employeeId": "e.employee_id", "channelId": "e.channel_id", "appId": "a.app_id", "accountId": "e.account_id"} {
		if v := q.Get(k); v != "" {
			where = append(where, col+"=?")
			args = append(args, v)
		}
	}
	if v := q.Get("keyword"); v != "" {
		p := "%" + v + "%"
		where = append(where, "(a.name LIKE ? OR a.uid LIKE ? OR a.bak_name LIKE ?)")
		args = append(args, p, p, p)
	}
	base := " FROM earnings e JOIN accounts a ON a.id=e.account_id LEFT JOIN employees emp ON emp.id=e.employee_id LEFT JOIN apps ap ON ap.id=a.app_id LEFT JOIN channels ch ON ch.id=e.channel_id WHERE " + strings.Join(where, " AND ")
	var total int
	var sum int64
	if e = s.db.QueryRow("SELECT count(*),COALESCE(SUM(e.amount_cents),0)"+base, args...).Scan(&total, &sum); e != nil {
		return nil, e
	}
	p, sz := page(q)
	sqlq := "SELECT e.id,e.account_id,e.employee_id,e.team_id,e.channel_id,e.day,e.hour,e.amount_cents,e.kind,e.source_ref,e.note,e.created_at,a.name,a.uid,a.bak_name,a.gift_account,emp.name,ap.name,ch.name" + base + " ORDER BY e.day DESC,e.id DESC"
	qa := append([]any{}, args...)
	if !all {
		sqlq += " LIMIT ? OFFSET ?"
		qa = append(qa, sz, (p-1)*sz)
	}
	rows, e := s.db.Query(sqlq, qa...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, aid, hour int
		var empID, teamID, chID sql.NullInt64
		var day, kind, src, note, created, name, uid, bak string
		var amount int64
		var gift int
		var emp, app, ch sql.NullString
		if e = rows.Scan(&id, &aid, &empID, &teamID, &chID, &day, &hour, &amount, &kind, &src, &note, &created, &name, &uid, &bak, &gift, &emp, &app, &ch); e != nil {
			return nil, e
		}
		list = append(list, map[string]any{"id": id, "accountId": aid, "employeeId": nullInt(empID), "teamId": nullInt(teamID), "channelId": nullInt(chID), "day": day, "hour": hour, "amount": money(amount), "kind": kind, "sourceRef": src, "note": note, "createDate": created, "chatAccountName": name, "chatAccountUid": uid, "chatAccountBakName": bak, "giftAccount": gift, "employeeName": nullString(emp), "appName": nullString(app), "channelName": nullString(ch)})
	}
	return map[string]any{"list": list, "total": total, "sum": money(sum), "page": p, "pageSize": sz, "startDate": start, "endDate": end}, nil
}
func nullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
func nullString(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}
func (s *Server) earnings(w http.ResponseWriter, r *http.Request, u *User) error {
	d, e := s.earningsData(u, r.URL.Query(), false)
	if e != nil {
		return e
	}
	writeJSON(w, 200, d)
	return nil
}
func (s *Server) addEarning(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct {
		AccountID int     `json:"accountId"`
		Day       string  `json:"day"`
		Hour      int     `json:"hour"`
		Amount    float64 `json:"amount"`
		SourceRef string  `json:"sourceRef"`
		Note      string  `json:"note"`
	}
	if e := decode(r, &p); e != nil {
		return e
	}
	if p.AccountID <= 0 || p.Amount <= 0 || len(p.SourceRef) < 8 {
		return fail(422, "Please check the submitted fields")
	}
	id, dedup, e := s.recordEarning(u, p.AccountID, p.Day, p.Hour, cents(p.Amount), p.SourceRef, p.Note, nil)
	if e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"id": id, "deduplicated": dedup})
	return nil
}
func (s *Server) adjustEarning(w http.ResponseWriter, r *http.Request, u *User) error {
	baseID, e := pathID(r)
	if e != nil {
		return e
	}
	var p struct {
		Amount            float64 `json:"amount"`
		SourceRef, Reason string
	}
	if e = decode(r, &p); e != nil {
		return e
	}
	if p.Amount == 0 || len(p.SourceRef) < 8 || len(p.Reason) < 3 {
		return fail(422, "Please check the submitted fields")
	}
	var aid int
	var day string
	var hour int
	if e = s.db.QueryRow("SELECT account_id,day,hour FROM earnings WHERE id=? AND company_id=?", baseID, u.CompanyID).Scan(&aid, &day, &hour); e == sql.ErrNoRows {
		return fail(404, "Record not found in your company")
	}
	if e != nil {
		return e
	}
	id, dedup, e := s.recordEarning(u, aid, day, hour, cents(p.Amount), p.SourceRef, p.Reason, &baseID)
	if e != nil {
		return e
	}
	writeJSON(w, 200, map[string]any{"id": id, "deduplicated": dedup})
	return nil
}
func (s *Server) recordEarning(u *User, aid int, day string, hour int, amount int64, ref, note string, adjust *int) (int64, bool, error) {
	if day == "" {
		day = time.Now().Format("2006-01-02")
	}
	dt, e := time.Parse("2006-01-02", day)
	if e != nil || dt.After(time.Now().Add(24*time.Hour)) {
		return 0, false, fail(422, "Future income dates are not allowed")
	}
	var ch int
	var emp, team sql.NullInt64
	if e = s.db.QueryRow("SELECT channel_id,employee_id,(SELECT team_id FROM employees WHERE id=accounts.employee_id) FROM accounts WHERE id=? AND company_id=?", aid, u.CompanyID).Scan(&ch, &emp, &team); e == sql.ErrNoRows {
		return 0, false, fail(404, "Record not found in your company")
	}
	if e != nil {
		return 0, false, e
	}
	var old int
	err := s.db.QueryRow("SELECT id FROM earnings WHERE company_id=? AND source='manual' AND source_ref=?", u.CompanyID, ref).Scan(&old)
	if err == nil {
		return int64(old), true, nil
	}
	kind := "income"
	if adjust != nil {
		kind = "adjustment"
	}
	res, e := s.db.Exec("INSERT INTO earnings(company_id,account_id,employee_id,team_id,channel_id,day,hour,amount_cents,kind,source_ref,note,created_by,adjusts_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)", u.CompanyID, aid, nullInt(emp), nullInt(team), ch, day, hour, amount, kind, ref, note, u.ID, adjust)
	if e != nil {
		return 0, false, dbConflict(e)
	}
	id, _ := res.LastInsertId()
	s.audit(u, kind, "earnings", id, map[string]any{"amountCents": amount, "accountId": aid})
	return id, false, nil
}
func (s *Server) withdrawalsData(u *User, q url.Values, all bool) (map[string]any, error) {
	where := []string{"w.company_id=?"}
	args := []any{u.CompanyID}
	for k, col := range map[string]string{"state": "w.state", "accountId": "w.account_id", "holderId": "w.holder_id", "channelId": "a.channel_id", "appId": "a.app_id"} {
		if v := q.Get(k); v != "" {
			where = append(where, col+"=?")
			args = append(args, v)
		}
	}
	if v := q.Get("keyword"); v != "" {
		p := "%" + v + "%"
		where = append(where, "(a.name LIKE ? OR a.bak_name LIKE ? OR a.uid LIKE ?)")
		args = append(args, p, p, p)
	}
	base := " FROM withdrawals w JOIN accounts a ON a.id=w.account_id WHERE " + strings.Join(where, " AND ")
	var total int
	if e := s.db.QueryRow("SELECT count(*)"+base, args...).Scan(&total); e != nil {
		return nil, e
	}
	p, sz := page(q)
	sqlq := "SELECT w.id,w.account_id,w.holder_id,w.amount_cents,w.fee_cents,w.holder_share_cents,w.percent_bps,w.state,w.note,w.review_note,w.payment_ref,w.created_at,w.paid_at,w.version,w.created_by,w.reviewed_by,a.name,a.uid,a.bak_name,ap.name,ch.name,h.name FROM withdrawals w JOIN accounts a ON a.id=w.account_id JOIN apps ap ON ap.id=a.app_id JOIN channels ch ON ch.id=a.channel_id LEFT JOIN holders h ON h.id=w.holder_id WHERE " + strings.Join(where, " AND ") + " ORDER BY w.id DESC"
	qa := append([]any{}, args...)
	if !all {
		sqlq += " LIMIT ? OFFSET ?"
		qa = append(qa, sz, (p-1)*sz)
	}
	rows, e := s.db.Query(sqlq, qa...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, aid, version, createdBy int
		var holder sql.NullInt64
		var amount, fee, share, pct int64
		var state, note, review, created, name, uid, bak, app, ch string
		var pref, paid, holderName sql.NullString
		var reviewed sql.NullInt64
		if e = rows.Scan(&id, &aid, &holder, &amount, &fee, &share, &pct, &state, &note, &review, &pref, &created, &paid, &version, &createdBy, &reviewed, &name, &uid, &bak, &app, &ch, &holderName); e != nil {
			return nil, e
		}
		list = append(list, map[string]any{"id": id, "accountId": aid, "holderId": nullInt(holder), "chatAccountName": name, "chatAccountUid": uid, "chatAccountBakName": bak, "appName": app, "channelName": ch, "holderName": nullString(holderName), "amount": money(amount), "fee": money(fee), "netAmount": money(amount - fee), "holderShare": money(share), "percent": money(pct), "state": state, "note": note, "reviewNote": review, "paymentRef": nullString(pref), "createDate": created, "paidAt": nullString(paid), "version": version, "createdBy": createdBy, "reviewedBy": nullInt(reviewed)})
	}
	return map[string]any{"list": list, "total": total, "page": p, "pageSize": sz}, nil
}
func (s *Server) withdrawals(w http.ResponseWriter, r *http.Request, u *User) error {
	d, e := s.withdrawalsData(u, r.URL.Query(), false)
	if e != nil {
		return e
	}
	writeJSON(w, 200, d)
	return nil
}
func (s *Server) addWithdrawal(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct {
		AccountID            int `json:"accountId"`
		Amount, Fee          float64
		IdempotencyKey, Note string
	}
	if e := decode(r, &p); e != nil {
		return e
	}
	if p.AccountID <= 0 || p.Amount <= 0 || p.Fee < 0 || p.Fee >= p.Amount || len(p.IdempotencyKey) < 8 {
		return fail(422, "Please check the submitted fields")
	}
	payload, _ := json.Marshal(p)
	ph := digest(string(payload))
	var oldID int
	var oldHash string
	err := s.db.QueryRow("SELECT id,payload_hash FROM withdrawals WHERE company_id=? AND idempotency_key=?", u.CompanyID, p.IdempotencyKey).Scan(&oldID, &oldHash)
	if err == nil {
		if oldHash != ph {
			return fail(409, "Idempotency key was reused with different data")
		}
		d, e := s.withdrawalsData(u, url.Values{"accountId": []string{strconv.Itoa(p.AccountID)}}, true)
		if e != nil {
			return e
		}
		for _, x := range d["list"].([]map[string]any) {
			if int(asInt(x["id"])) == oldID {
				writeJSON(w, 200, x)
				return nil
			}
		}
	}
	var status int
	var opening int64
	var holder sql.NullInt64
	err = s.db.QueryRow("SELECT status,opening_cents,holder_id FROM accounts WHERE id=? AND company_id=?", p.AccountID, u.CompanyID).Scan(&status, &opening, &holder)
	if err == sql.ErrNoRows {
		return fail(404, "Record not found in your company")
	}
	if err != nil {
		return err
	}
	if status != 0 {
		return fail(409, "Cannot request withdrawal for an inactive account")
	}
	_, available, _, e := s.accountBalances(p.AccountID, u.CompanyID, opening)
	if e != nil {
		return e
	}
	amt, fee := cents(p.Amount), cents(p.Fee)
	if amt > available {
		return fail(409, "Insufficient available balance (pending withdrawals reserve funds)")
	}
	var hc, pct sql.NullInt64
	if holder.Valid {
		_ = s.db.QueryRow("SELECT holder_channel_id,percent_bps FROM holders WHERE id=? AND company_id=?", holder.Int64, u.CompanyID).Scan(&hc, &pct)
	}
	share := int64(math.Round(float64(amt-fee) * float64(pct.Int64) / 10000))
	res, e := s.db.Exec("INSERT INTO withdrawals(company_id,account_id,holder_id,holder_channel_id,amount_cents,fee_cents,percent_bps,holder_share_cents,idempotency_key,payload_hash,note,created_by) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", u.CompanyID, p.AccountID, nullInt(holder), nullInt(hc), amt, fee, pct.Int64, share, p.IdempotencyKey, ph, p.Note, u.ID)
	if e != nil {
		return dbConflict(e)
	}
	id, _ := res.LastInsertId()
	s.audit(u, "request", "withdrawals", id, map[string]any{"amountCents": amt, "accountId": p.AccountID})
	d, e := s.withdrawalsData(u, url.Values{}, true)
	if e != nil {
		return e
	}
	for _, x := range d["list"].([]map[string]any) {
		if asInt(x["id"]) == id {
			writeJSON(w, 200, x)
			return nil
		}
	}
	return fail(500, "Created withdrawal could not be read")
}
func (s *Server) transitionWithdrawal(w http.ResponseWriter, r *http.Request, u *User) error {
	id, e := pathID(r)
	if e != nil {
		return e
	}
	var p struct {
		Action             string
		Version            int
		Reason, PaymentRef string
	}
	if e = decode(r, &p); e != nil {
		return e
	}
	if !contains([]string{"approve", "reject", "pay"}, p.Action) {
		return fail(422, "Please check the submitted fields")
	}
	if p.Action == "pay" && len(p.PaymentRef) < 6 {
		return fail(422, "An external payment reference of at least 6 characters is required")
	}
	if p.Action == "reject" && len(p.Reason) < 3 {
		return fail(422, "A rejection reason of at least 3 characters is required")
	}
	var state string
	var ver, createdBy int
	var review, pref sql.NullString
	if e = s.db.QueryRow("SELECT state,version,created_by,review_note,payment_ref FROM withdrawals WHERE id=? AND company_id=?", id, u.CompanyID).Scan(&state, &ver, &createdBy, &review, &pref); e == sql.ErrNoRows {
		return fail(404, "Record not found in your company")
	}
	if e != nil {
		return e
	}
	if ver != p.Version {
		return fail(409, "Withdrawal changed; refresh before retrying")
	}
	next := ""
	switch state + ":" + p.Action {
	case "pending:approve":
		next = "approved"
	case "pending:reject", "approved:reject":
		next = "rejected"
	case "approved:pay":
		next = "paid"
	}
	if next == "" {
		return fail(409, "Invalid withdrawal state transition")
	}
	if p.Action == "approve" && createdBy == u.ID {
		return fail(403, "A different finance/admin user must review this withdrawal")
	}
	if p.Reason == "" {
		p.Reason = review.String
	}
	if p.PaymentRef == "" {
		p.PaymentRef = pref.String
	}
	_, e = s.db.Exec("UPDATE withdrawals SET state=?,reviewed_by=CASE WHEN ? IN ('approve','reject') THEN ? ELSE reviewed_by END,review_note=?,payment_ref=?,paid_at=CASE WHEN ?='paid' THEN CURRENT_TIMESTAMP ELSE paid_at END,updated_at=CURRENT_TIMESTAMP,version=version+1 WHERE id=? AND company_id=?", next, p.Action, u.ID, p.Reason, nullStr(p.PaymentRef), next, id, u.CompanyID)
	if e != nil {
		return dbConflict(e)
	}
	s.audit(u, p.Action, "withdrawals", id, map[string]any{"from": state, "to": next})
	d, e := s.withdrawalsData(u, url.Values{}, true)
	if e != nil {
		return e
	}
	for _, x := range d["list"].([]map[string]any) {
		if int(asInt(x["id"])) == id {
			writeJSON(w, 200, x)
			return nil
		}
	}
	return fail(500, "Withdrawal could not be read")
}

func (s *Server) settlements(w http.ResponseWriter, r *http.Request, u *User) error {
	where := "s.company_id=?"
	args := []any{u.CompanyID}
	if st := r.URL.Query().Get("state"); st != "" {
		where += " AND s.state=?"
		args = append(args, st)
	}
	var total int
	_ = s.db.QueryRow("SELECT count(*) FROM settlements s WHERE "+where, args...).Scan(&total)
	p, sz := page(r.URL.Query())
	rows, e := s.db.Query("SELECT s.id,s.holder_id,h.name,s.amount_cents,s.state,s.start_date,s.end_date,s.payment_ref,s.created_at,s.paid_at,s.version FROM settlements s JOIN holders h ON h.id=s.holder_id WHERE "+where+" ORDER BY s.id DESC LIMIT ? OFFSET ?", append(args, sz, (p-1)*sz)...)
	if e != nil {
		return e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, hid, ver int
		var name, state, start, end, created string
		var amt int64
		var pref, paid sql.NullString
		if e = rows.Scan(&id, &hid, &name, &amt, &state, &start, &end, &pref, &created, &paid, &ver); e != nil {
			return e
		}
		list = append(list, map[string]any{"id": id, "holderId": hid, "holderName": name, "amount": money(amt), "state": state, "startDate": start, "endDate": end, "paymentRef": nullString(pref), "createDate": created, "paidAt": nullString(paid), "version": ver})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": total, "page": p, "pageSize": sz})
	return nil
}
func (s *Server) addSettlement(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct {
		HolderID                           int `json:"holderId"`
		StartDate, EndDate, IdempotencyKey string
	}
	if e := decode(r, &p); e != nil {
		return e
	}
	if p.HolderID <= 0 || len(p.IdempotencyKey) < 8 || p.StartDate > p.EndDate {
		return fail(422, "Please check the submitted fields")
	}
	payload, _ := json.Marshal(p)
	ph := digest(string(payload))
	var oldID int
	var oldHash string
	var oldAmt int64
	if e := s.db.QueryRow("SELECT id,payload_hash,amount_cents FROM settlements WHERE company_id=? AND idempotency_key=?", u.CompanyID, p.IdempotencyKey).Scan(&oldID, &oldHash, &oldAmt); e == nil {
		if oldHash != ph {
			return fail(409, "Idempotency key was reused with different data")
		}
		writeJSON(w, 200, map[string]any{"id": oldID, "amount": money(oldAmt), "deduplicated": true})
		return nil
	}
	rows, e := s.db.Query("SELECT w.id,w.holder_share_cents FROM withdrawals w WHERE w.company_id=? AND w.holder_id=? AND w.state='paid' AND date(w.paid_at) BETWEEN ? AND ? AND NOT EXISTS (SELECT 1 FROM settlement_items i WHERE i.withdrawal_id=w.id)", u.CompanyID, p.HolderID, p.StartDate, p.EndDate)
	if e != nil {
		return e
	}
	type item struct {
		id    int
		share int64
	}
	var items []item
	var total int64
	for rows.Next() {
		var x item
		_ = rows.Scan(&x.id, &x.share)
		items = append(items, x)
		total += x.share
	}
	rows.Close()
	if len(items) == 0 {
		return fail(409, "No paid, unsettled withdrawals in this period")
	}
	tx, e := s.db.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	res, e := tx.Exec("INSERT INTO settlements(company_id,holder_id,amount_cents,start_date,end_date,idempotency_key,payload_hash,created_by) VALUES (?,?,?,?,?,?,?,?)", u.CompanyID, p.HolderID, total, p.StartDate, p.EndDate, p.IdempotencyKey, ph, u.ID)
	if e != nil {
		return dbConflict(e)
	}
	sid, _ := res.LastInsertId()
	for _, x := range items {
		if _, e = tx.Exec("INSERT INTO settlement_items(settlement_id,withdrawal_id,share_cents) VALUES (?,?,?)", sid, x.id, x.share); e != nil {
			return dbConflict(e)
		}
	}
	if e = tx.Commit(); e != nil {
		return e
	}
	s.audit(u, "create", "settlements", sid, map[string]any{"amountCents": total, "items": len(items)})
	writeJSON(w, 200, map[string]any{"id": sid, "amount": money(total), "deduplicated": false})
	return nil
}
func (s *Server) paySettlement(w http.ResponseWriter, r *http.Request, u *User) error {
	id, e := pathID(r)
	if e != nil {
		return e
	}
	var p struct {
		Version    int
		PaymentRef string
	}
	if e = decode(r, &p); e != nil {
		return e
	}
	if len(p.PaymentRef) < 6 {
		return fail(422, "Please check the submitted fields")
	}
	var state string
	var ver int
	if e = s.db.QueryRow("SELECT state,version FROM settlements WHERE id=? AND company_id=?", id, u.CompanyID).Scan(&state, &ver); e == sql.ErrNoRows {
		return fail(404, "Record not found in your company")
	}
	if e != nil {
		return e
	}
	if ver != p.Version {
		return fail(409, "Settlement changed; refresh before retrying")
	}
	if state != "draft" {
		return fail(409, "Settlement is already paid")
	}
	_, e = s.db.Exec("UPDATE settlements SET state='paid',payment_ref=?,paid_by=?,paid_at=CURRENT_TIMESTAMP,version=version+1 WHERE id=? AND company_id=?", p.PaymentRef, u.ID, id, u.CompanyID)
	if e != nil {
		return dbConflict(e)
	}
	s.audit(u, "pay", "settlements", id, map[string]any{"paymentRef": p.PaymentRef})
	writeJSON(w, 200, map[string]any{"paid": true})
	return nil
}
func (s *Server) reconciliation(w http.ResponseWriter, r *http.Request, u *User) error {
	rows, e := s.db.Query(`SELECT h.id,h.name,h.bak_name,c.name,h.percent_bps,
COALESCE(SUM(CASE WHEN w.state='paid' THEN w.amount_cents ELSE 0 END),0),
COALESCE(SUM(CASE WHEN w.state='paid' AND si.withdrawal_id IS NULL THEN w.holder_share_cents ELSE 0 END),0),
COALESCE((SELECT SUM(amount_cents) FROM settlements s2 WHERE s2.holder_id=h.id AND s2.company_id=h.company_id AND s2.state='draft'),0),
COALESCE((SELECT SUM(amount_cents) FROM settlements s3 WHERE s3.holder_id=h.id AND s3.company_id=h.company_id AND s3.state='paid'),0),h.channel_id
FROM holders h JOIN channels c ON c.id=h.channel_id LEFT JOIN withdrawals w ON w.holder_id=h.id LEFT JOIN settlement_items si ON si.withdrawal_id=w.id WHERE h.company_id=? GROUP BY h.id ORDER BY h.id`, u.CompanyID)
	if e != nil {
		return e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id, ch int
		var name, bak, channel string
		var pct, paid, unbilled, waiting, finished int64
		if e = rows.Scan(&id, &name, &bak, &channel, &pct, &paid, &unbilled, &waiting, &finished, &ch); e != nil {
			return e
		}
		list = append(list, map[string]any{"id": id, "name": name, "bakName": bak, "channelName": channel, "channelId": ch, "percent": money(pct), "paidTotal": money(paid), "unbilledShare": money(unbilled), "waitingPayTotal": money(waiting), "finishedTotal": money(finished)})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": len(list)})
	return nil
}
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request, u *User) error {
	start, end, e := dateRange(r.URL.Query(), 7)
	if e != nil {
		return e
	}
	args := []any{u.CompanyID}
	cond := "a.company_id=?"
	if ch := r.URL.Query().Get("channelId"); ch != "" {
		cond += " AND a.channel_id=?"
		args = append(args, ch)
	}
	rows, e := queryMaps(s.db, "SELECT a.* FROM accounts a WHERE "+cond, args...)
	if e != nil {
		return e
	}
	var bal, av, held int64
	active := 0
	for _, a := range rows {
		b, x, h, e := s.accountBalances(int(asInt(a["id"])), u.CompanyID, asInt(a["opening_cents"]))
		if e != nil {
			return e
		}
		bal += b
		av += x
		held += h
		if asInt(a["status"]) == 0 {
			active++
		}
	}
	var income, prev, withdrawn int64
	ebase := " FROM earnings e JOIN accounts a ON a.id=e.account_id WHERE " + cond
	wbase := " FROM withdrawals w JOIN accounts a ON a.id=w.account_id WHERE " + cond
	ea := append(append([]any{}, args...), start, end)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(e.amount_cents),0)"+ebase+" AND e.day BETWEEN ? AND ?", ea...).Scan(&income)
	st, _ := time.Parse("2006-01-02", start)
	et, _ := time.Parse("2006-01-02", end)
	days := int(et.Sub(st).Hours()/24) + 1
	ps := st.AddDate(0, 0, -days).Format("2006-01-02")
	pe := st.AddDate(0, 0, -1).Format("2006-01-02")
	pa := append(append([]any{}, args...), ps, pe)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(e.amount_cents),0)"+ebase+" AND e.day BETWEEN ? AND ?", pa...).Scan(&prev)
	wa := append(append([]any{}, args...), start, end)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(w.amount_cents),0)"+wbase+" AND w.state='paid' AND date(w.paid_at) BETWEEN ? AND ?", wa...).Scan(&withdrawn)
	series := []map[string]any{}
	for d := st; !d.After(et); d = d.AddDate(0, 0, 1) {
		day := d.Format("2006-01-02")
		var inc, wd int64
		qa := append(append([]any{}, args...), day)
		_ = s.db.QueryRow("SELECT COALESCE(SUM(e.amount_cents),0)"+ebase+" AND e.day=?", qa...).Scan(&inc)
		_ = s.db.QueryRow("SELECT COALESCE(SUM(w.amount_cents),0)"+wbase+" AND w.state='paid' AND date(w.paid_at)=?", qa...).Scan(&wd)
		series = append(series, map[string]any{"day": day, "income": money(inc), "withdraw": money(wd), "balance": 0})
	}
	rankArgs := append(append([]any{}, args...), start, end)
	rankRows, e := queryMaps(s.db, "SELECT e.employee_id employeeId,COALESCE(emp.name,'Unassigned') employeeName,SUM(e.amount_cents) cents FROM earnings e JOIN accounts a ON a.id=e.account_id LEFT JOIN employees emp ON emp.id=e.employee_id WHERE "+cond+" AND e.day BETWEEN ? AND ? GROUP BY e.employee_id,emp.name ORDER BY cents DESC LIMIT 6", rankArgs...)
	if e != nil {
		return e
	}
	ranking := []map[string]any{}
	for _, x := range rankRows {
		ranking = append(ranking, map[string]any{"employeeId": x["employeeId"], "employeeName": x["employeeName"], "amount": money(asInt(x["cents"]))})
	}
	var empCount, pending int
	empq := "SELECT count(*) FROM employees WHERE company_id=? AND status=0"
	empa := []any{u.CompanyID}
	if ch := r.URL.Query().Get("channelId"); ch != "" {
		empq += " AND channel_id=?"
		empa = append(empa, ch)
	}
	_ = s.db.QueryRow(empq, empa...).Scan(&empCount)
	_ = s.db.QueryRow("SELECT count(*)"+wbase+" AND w.state IN ('pending','approved')", args...).Scan(&pending)
	md, e := s.monitorData(u, r.URL.Query())
	if e != nil {
		return e
	}
	alerts := md["summary"]
	writeJSON(w, 200, map[string]any{"balance": money(bal), "available": money(av), "reserved": money(held), "income": money(income), "previousIncome": money(prev), "withdrawn": money(withdrawn), "accountCount": len(rows), "activeAccounts": active, "employeeCount": empCount, "pendingCount": pending, "startDate": start, "endDate": end, "series": series, "ranking": ranking, "alerts": alerts, "isDemo": true})
	return nil
}
func (s *Server) monitorData(u *User, q url.Values) (map[string]any, error) {
	d, e := s.entityList("accounts", u, q, true)
	if e != nil {
		return nil, e
	}
	today := time.Now().Format("2006-01-02")
	seen := map[int]bool{}
	rows, e := s.db.Query("SELECT DISTINCT account_id FROM earnings WHERE company_id=? AND day=?", u.CompanyID, today)
	if e != nil {
		return nil, e
	}
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		seen[id] = true
	}
	rows.Close()
	all := d["list"].([]map[string]any)
	out := []map[string]any{}
	summary := map[string]int{"banned": 0, "missing_income": 0, "no_employee": 0, "no_holder": 0, "stale_heartbeat": 0}
	for _, x := range all {
		alerts := []string{}
		st := asInt(x["accountStatus"])
		id := int(asInt(x["id"]))
		if st == 1 {
			alerts = append(alerts, "banned")
		} else if st == 2 {
			alerts = append(alerts, "closed")
		} else {
			if !seen[id] {
				alerts = append(alerts, "missing_income")
			}
			if x["employeeId"] == nil || asInt(x["employeeId"]) == 0 {
				alerts = append(alerts, "no_employee")
			}
			if x["accountHolderId"] == nil || asInt(x["accountHolderId"]) == 0 {
				alerts = append(alerts, "no_holder")
			}
			last := strv(x["lastSeen"])
			tt, er := time.Parse("2006-01-02 15:04:05", last)
			if er != nil || time.Since(tt) > 48*time.Hour {
				alerts = append(alerts, "stale_heartbeat")
			}
		}
		x["alerts"] = alerts
		if len(alerts) > 0 {
			x["health"] = "attention"
		} else {
			x["health"] = "healthy"
		}
		for k := range summary {
			if contains(alerts, k) {
				summary[k]++
			}
		}
		if a := q.Get("alert"); a != "" && !contains(alerts, a) {
			continue
		}
		out = append(out, x)
	}
	p, sz := page(q)
	start := (p - 1) * sz
	if start > len(out) {
		start = len(out)
	}
	end := start + sz
	if end > len(out) {
		end = len(out)
	}
	return map[string]any{"list": out[start:end], "total": len(out), "summary": summary, "page": p, "pageSize": sz, "truncated": false, "heartbeatSource": "manual-or-authenticated-api", "verificationDeviceSupport": false}, nil
}
func (s *Server) monitor(w http.ResponseWriter, r *http.Request, u *User) error {
	d, e := s.monitorData(u, r.URL.Query())
	if e != nil {
		return e
	}
	writeJSON(w, 200, d)
	return nil
}
func (s *Server) report(w http.ResponseWriter, r *http.Request, u *User) error {
	kind := r.PathValue("kind")
	switch kind {
	case "employees", "teams", "hourly":
		return s.incomeReport(w, r, u, kind)
	case "comparison":
		return s.compareReport(w, r, u)
	case "bans":
		return s.bansReport(w, r, u)
	}
	return fail(404, "Unknown report")
}
func (s *Server) incomeReport(w http.ResponseWriter, r *http.Request, u *User, kind string) error {
	start, end, e := dateRange(r.URL.Query(), 7)
	if e != nil {
		return e
	}
	where := []string{"e.company_id=?", "e.day BETWEEN ? AND ?"}
	args := []any{u.CompanyID, start, end}
	if ch := r.URL.Query().Get("channelId"); ch != "" {
		where = append(where, "e.channel_id=?")
		args = append(args, ch)
	}
	base := " FROM earnings e JOIN accounts a ON a.id=e.account_id LEFT JOIN employees emp ON emp.id=e.employee_id LEFT JOIN teams t ON t.id=e.team_id WHERE " + strings.Join(where, " AND ")
	selectx, group := "", ""
	switch kind {
	case "employees":
		selectx = "e.employee_id id,COALESCE(emp.name,'Unassigned') name"
		group = "e.employee_id,emp.name"
	case "teams":
		selectx = "e.team_id id,COALESCE(t.name,'Unassigned') name"
		group = "e.team_id,t.name"
	case "hourly":
		selectx = "e.hour id,LPAD(e.hour::text,2,'0') || ':00' name"
		group = "e.hour"
	}
	rows, e := queryMaps(s.db, "SELECT "+selectx+",SUM(e.amount_cents) total,COUNT(DISTINCT e.account_id) accountCount,COUNT(*) entryCount"+base+" GROUP BY "+group+" ORDER BY total DESC", args...)
	if e != nil {
		return e
	}
	sum := 0.0
	list := []map[string]any{}
	for _, x := range rows {
		amt := money(asInt(x["total"]))
		sum += amt
		list = append(list, map[string]any{"id": x["id"], "name": x["name"], "amount": amt, "accountCount": x["accountCount"], "entryCount": x["entryCount"]})
	}
	p, sz := page(r.URL.Query())
	a := (p - 1) * sz
	if a > len(list) {
		a = len(list)
	}
	b := a + sz
	if b > len(list) {
		b = len(list)
	}
	writeJSON(w, 200, map[string]any{"list": list[a:b], "total": len(list), "sum": sum, "startDate": start, "endDate": end, "page": p, "pageSize": sz})
	return nil
}
func (s *Server) compareReport(w http.ResponseWriter, r *http.Request, u *User) error {
	start, end, e := dateRange(r.URL.Query(), 1)
	if e != nil {
		return e
	}
	st, _ := time.Parse("2006-01-02", start)
	et, _ := time.Parse("2006-01-02", end)
	days := int(et.Sub(st).Hours()/24) + 1
	ps := st.AddDate(0, 0, -days).Format("2006-01-02")
	pe := st.AddDate(0, 0, -1).Format("2006-01-02")
	ed, e := s.entityList("accounts", u, r.URL.Query(), true)
	if e != nil {
		return e
	}
	list := []map[string]any{}
	for _, a := range ed["list"].([]map[string]any) {
		id := int(asInt(a["id"]))
		var cur, prev int64
		_ = s.db.QueryRow("SELECT COALESCE(SUM(CASE WHEN day BETWEEN ? AND ? THEN amount_cents ELSE 0 END),0),COALESCE(SUM(CASE WHEN day BETWEEN ? AND ? THEN amount_cents ELSE 0 END),0) FROM earnings WHERE company_id=? AND account_id=?", start, end, ps, pe, u.CompanyID, id).Scan(&cur, &prev)
		var pct any
		if prev != 0 {
			pct = math.Round((float64(cur-prev)/float64(prev)*100)*100) / 100
		}
		list = append(list, map[string]any{"id": id, "chatAccountName": a["chatAccountName"], "chatAccountBakName": a["chatAccountBakName"], "appName": a["appName"], "employeeName": a["employeeName"], "current": money(cur), "previous": money(prev), "difference": money(cur - prev), "changePercent": pct})
	}
	p, sz := page(r.URL.Query())
	a := (p - 1) * sz
	if a > len(list) {
		a = len(list)
	}
	b := a + sz
	if b > len(list) {
		b = len(list)
	}
	writeJSON(w, 200, map[string]any{"list": list[a:b], "total": len(list), "startDate": start, "endDate": end, "previousStartDate": ps, "previousEndDate": pe, "page": p, "pageSize": sz})
	return nil
}
func (s *Server) bansReport(w http.ResponseWriter, r *http.Request, u *User) error {
	start, end, e := dateRange(r.URL.Query(), 30)
	if e != nil {
		return e
	}
	rows, e := s.db.Query("SELECT b.id,b.account_id,a.name,a.uid,a.bak_name,ap.name,emp.name,b.reason,b.balance_cents,b.created_at FROM bans b JOIN accounts a ON a.id=b.account_id JOIN apps ap ON ap.id=a.app_id LEFT JOIN employees emp ON emp.id=b.employee_id WHERE b.company_id=? AND date(b.created_at) BETWEEN ? AND ? ORDER BY b.id DESC", u.CompanyID, start, end)
	if e != nil {
		return e
	}
	defer rows.Close()
	list := []map[string]any{}
	var total int64
	for rows.Next() {
		var id, aid int
		var name, uid, bak, app, reason, created string
		var emp sql.NullString
		var bal int64
		if e = rows.Scan(&id, &aid, &name, &uid, &bak, &app, &emp, &reason, &bal, &created); e != nil {
			return e
		}
		total += bal
		list = append(list, map[string]any{"id": id, "accountId": aid, "chatAccountName": name, "chatAccountUid": uid, "chatAccountBakName": bak, "appName": app, "employeeName": nullString(emp), "reason": reason, "balanceAtBan": money(bal), "createDate": created})
	}
	writeJSON(w, 200, map[string]any{"list": list, "total": len(list), "balanceAtBanTotal": money(total)})
	return nil
}
func (s *Server) auditLogs(w http.ResponseWriter, r *http.Request, u *User) error {
	p, sz := page(r.URL.Query())
	rows, e := s.db.Query("SELECT a.id,u.display_name,a.action,a.entity,a.entity_id,a.detail,a.created_at FROM audit_logs a JOIN users u ON u.id=a.user_id WHERE a.company_id=? ORDER BY a.id DESC LIMIT ? OFFSET ?", u.CompanyID, sz, (p-1)*sz)
	if e != nil {
		return e
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id int
		var actor, action, entity, detail, created string
		var eid sql.NullInt64
		if e = rows.Scan(&id, &actor, &action, &entity, &eid, &detail, &created); e != nil {
			return e
		}
		list = append(list, map[string]any{"id": id, "actor": actor, "action": action, "entity": entity, "entityId": nullInt(eid), "detail": detail, "createDate": created})
	}
	var total int
	_ = s.db.QueryRow("SELECT count(*) FROM audit_logs WHERE company_id=?", u.CompanyID).Scan(&total)
	writeJSON(w, 200, map[string]any{"list": list, "total": total, "page": p, "pageSize": sz})
	return nil
}
func (s *Server) banCore(w http.ResponseWriter, r *http.Request, u *User, unban bool) error {
	id, e := pathID(r)
	if e != nil {
		return e
	}
	var p struct {
		Reason  string
		Version int
	}
	if e = decode(r, &p); e != nil {
		return e
	}
	if len(p.Reason) < 3 {
		return fail(422, "Please check the submitted fields")
	}
	var status, ver int
	var opening int64
	var emp sql.NullInt64
	if e = s.db.QueryRow("SELECT status,version,opening_cents,employee_id FROM accounts WHERE id=? AND company_id=?", id, u.CompanyID).Scan(&status, &ver, &opening, &emp); e == sql.ErrNoRows {
		return fail(404, "Record not found in your company")
	}
	if e != nil {
		return e
	}
	if ver != p.Version {
		return fail(409, "Record changed; refresh before retrying")
	}
	next := 1
	if unban {
		next = 0
	}
	if _, e = s.db.Exec("UPDATE accounts SET status=?,version=version+1 WHERE id=? AND company_id=?", next, id, u.CompanyID); e != nil {
		return e
	}
	if !unban {
		bal, _, _, _ := s.accountBalances(id, u.CompanyID, opening)
		_, _ = s.db.Exec("INSERT INTO bans(company_id,account_id,employee_id,reason,balance_cents,created_by) VALUES (?,?,?,?,?,?)", u.CompanyID, id, nullInt(emp), p.Reason, bal, u.ID)
	}
	act := "ban"
	if unban {
		act = "unban"
	}
	s.audit(u, act, "accounts", id, map[string]any{"reason": p.Reason})
	writeJSON(w, 200, map[string]any{"status": next})
	return nil
}
func (s *Server) banAccount(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.banCore(w, r, u, false)
}
func (s *Server) unbanAccount(w http.ResponseWriter, r *http.Request, u *User) error {
	return s.banCore(w, r, u, true)
}
func (s *Server) heartbeat(w http.ResponseWriter, r *http.Request, u *User) error {
	var p struct {
		AccountID int `json:"accountId"`
	}
	if e := decode(r, &p); e != nil {
		return e
	}
	res, e := s.db.Exec("UPDATE accounts SET last_seen=CURRENT_TIMESTAMP WHERE id=? AND company_id=?", p.AccountID, u.CompanyID)
	if e != nil {
		return e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fail(404, "Record not found in your company")
	}
	s.audit(u, "heartbeat", "accounts", p.AccountID, map[string]any{})
	writeJSON(w, 200, map[string]any{"received": true})
	return nil
}
func (s *Server) exportCSV(w http.ResponseWriter, r *http.Request, u *User) error {
	resource := r.PathValue("resource")
	var list []map[string]any
	switch resource {
	case "earnings":
		d, e := s.earningsData(u, r.URL.Query(), true)
		if e != nil {
			return e
		}
		list = d["list"].([]map[string]any)
	case "withdrawals":
		d, e := s.withdrawalsData(u, r.URL.Query(), true)
		if e != nil {
			return e
		}
		list = d["list"].([]map[string]any)
	case "employees", "accounts":
		d, e := s.entityList(resource, u, r.URL.Query(), true)
		if e != nil {
			return e
		}
		list = d["list"].([]map[string]any)
	default:
		return fail(404, "This export is not supported")
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s.csv\"", resource))
	_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if len(list) == 0 {
		_ = cw.Write([]string{"id"})
		return nil
	}
	keys := make([]string, 0, len(list[0]))
	for k := range list[0] {
		keys = append(keys, k)
	}
	_ = cw.Write(keys)
	for _, row := range list {
		rec := make([]string, len(keys))
		for i, k := range keys {
			v := strv(row[k])
			trim := strings.TrimLeft(v, " ")
			if strings.HasPrefix(trim, "=") || strings.HasPrefix(trim, "+") || strings.HasPrefix(trim, "-") || strings.HasPrefix(trim, "@") {
				v = "'" + v
			}
			rec[i] = v
		}
		_ = cw.Write(rec)
	}
	s.audit(u, "export", resource, nil, map[string]any{"rowCount": len(list)})
	return nil
}
