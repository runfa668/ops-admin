package app

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DB struct{ *sql.DB }
type Tx struct{ *sql.Tx }

func openPostgres(databaseURL string) (*DB, error) {
	raw, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}
	raw.SetMaxOpenConns(8)
	raw.SetMaxIdleConns(4)
	return &DB{raw}, nil
}

func rebind(q string) string {
	var b strings.Builder
	n := 1
	inSingle := false
	inDouble := false
	for i := 0; i < len(q); i++ {
		c := q[i]
		if c == '\'' && !inDouble {
			b.WriteByte(c)
			if inSingle && i+1 < len(q) && q[i+1] == '\'' {
				b.WriteByte(q[i+1])
				i++
				continue
			}
			inSingle = !inSingle
			continue
		}
		if c == '"' && !inSingle {
			inDouble = !inDouble
			b.WriteByte(c)
			continue
		}
		if c == '?' && !inSingle && !inDouble {
			b.WriteString(fmt.Sprintf("$%d", n))
			n++
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

type insertResult struct{ id int64 }

func (r insertResult) LastInsertId() (int64, error) { return r.id, nil }
func (r insertResult) RowsAffected() (int64, error) { return 1, nil }

var idTables = map[string]bool{
	"companies": true, "users": true, "login_attempts": true, "channels": true, "apps": true, "teams": true, "employees": true,
	"holder_channels": true, "holders": true, "accounts": true, "earnings": true, "withdrawals": true, "settlements": true,
	"blacklist": true, "configs": true, "bans": true, "audit_logs": true,
}

func insertTable(q string) string {
	f := strings.Fields(strings.TrimSpace(q))
	if len(f) >= 3 && strings.EqualFold(f[0], "insert") && strings.EqualFold(f[1], "into") {
		table := strings.Trim(f[2], "\"`")
		if i := strings.IndexByte(table, '('); i >= 0 {
			table = table[:i]
		}
		return strings.ToLower(strings.Trim(table, "\"`"))
	}
	return ""
}

func (d *DB) Exec(q string, args ...any) (sql.Result, error) {
	rq := rebind(q)
	if table := insertTable(q); idTables[table] && !strings.Contains(strings.ToLower(q), "returning") && !strings.Contains(strings.ToLower(q), " on conflict ") {
		var id int64
		if err := d.DB.QueryRow(rq+" RETURNING id", args...).Scan(&id); err != nil {
			return nil, err
		}
		return insertResult{id: id}, nil
	}
	return d.DB.Exec(rq, args...)
}
func (d *DB) Query(q string, args ...any) (*sql.Rows, error) { return d.DB.Query(rebind(q), args...) }
func (d *DB) QueryRow(q string, args ...any) *sql.Row        { return d.DB.QueryRow(rebind(q), args...) }
func (d *DB) Begin() (*Tx, error) {
	tx, err := d.DB.Begin()
	if err != nil {
		return nil, err
	}
	return &Tx{tx}, nil
}
func (d *DB) PingContext(ctx context.Context) error { return d.DB.PingContext(ctx) }

func (t *Tx) Exec(q string, args ...any) (sql.Result, error) {
	rq := rebind(q)
	if table := insertTable(q); idTables[table] && !strings.Contains(strings.ToLower(q), "returning") && !strings.Contains(strings.ToLower(q), " on conflict ") {
		var id int64
		if err := t.Tx.QueryRow(rq+" RETURNING id", args...).Scan(&id); err != nil {
			return nil, err
		}
		return insertResult{id: id}, nil
	}
	return t.Tx.Exec(rq, args...)
}
func (t *Tx) Query(q string, args ...any) (*sql.Rows, error) { return t.Tx.Query(rebind(q), args...) }
func (t *Tx) QueryRow(q string, args ...any) *sql.Row        { return t.Tx.QueryRow(rebind(q), args...) }
