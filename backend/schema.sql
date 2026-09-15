CREATE TABLE IF NOT EXISTS schema_version (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text));
INSERT INTO schema_version(version) VALUES (1) ON CONFLICT (version) DO NOTHING;
CREATE TABLE IF NOT EXISTS companies (id BIGSERIAL PRIMARY KEY, name TEXT NOT NULL UNIQUE);
CREATE TABLE IF NOT EXISTS users (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), username TEXT NOT NULL UNIQUE,
 display_name TEXT NOT NULL, password_hash TEXT NOT NULL, role TEXT NOT NULL CHECK(role IN ('admin','operator','finance','viewer')),
 active INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text)
);
CREATE TABLE IF NOT EXISTS sessions (
 token_hash TEXT PRIMARY KEY, user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 csrf TEXT NOT NULL, expires_at BIGINT NOT NULL, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text)
);
CREATE INDEX IF NOT EXISTS ix_session_expiry ON sessions(expires_at);
CREATE TABLE IF NOT EXISTS login_attempts (id BIGSERIAL PRIMARY KEY, bucket TEXT NOT NULL, created_at BIGINT NOT NULL);
CREATE INDEX IF NOT EXISTS ix_login_attempt ON login_attempts(bucket, created_at);
CREATE TABLE IF NOT EXISTS channels (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), name TEXT NOT NULL,
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text),
 version INTEGER NOT NULL DEFAULT 1, UNIQUE(company_id,name)
);
CREATE TABLE IF NOT EXISTS apps (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), name TEXT NOT NULL, package_name TEXT NOT NULL DEFAULT '',
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,name)
);
CREATE TABLE IF NOT EXISTS teams (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 name TEXT NOT NULL, leader_id BIGINT,
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,channel_id,name)
);
CREATE TABLE IF NOT EXISTS employees (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 team_id BIGINT REFERENCES teams(id) ON DELETE RESTRICT, name TEXT NOT NULL,
 status INTEGER NOT NULL DEFAULT 0 CHECK(status IN (0,1)), created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,channel_id,name)
);
CREATE TABLE IF NOT EXISTS holder_channels (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 name TEXT NOT NULL, percent_bps INTEGER NOT NULL DEFAULT 0 CHECK(percent_bps BETWEEN 0 AND 10000), account_valid_days INTEGER NOT NULL DEFAULT 30,
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,channel_id,name)
);
CREATE TABLE IF NOT EXISTS holders (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 holder_channel_id BIGINT REFERENCES holder_channels(id), name TEXT NOT NULL, bak_name TEXT NOT NULL DEFAULT '',
 percent_bps INTEGER NOT NULL DEFAULT 1000 CHECK(percent_bps BETWEEN 0 AND 10000),
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,channel_id,name)
);
CREATE TABLE IF NOT EXISTS accounts (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 app_id BIGINT NOT NULL REFERENCES apps(id), employee_id BIGINT REFERENCES employees(id), holder_id BIGINT REFERENCES holders(id),
 uid TEXT NOT NULL, name TEXT NOT NULL, bak_name TEXT NOT NULL DEFAULT '', status INTEGER NOT NULL DEFAULT 0 CHECK(status IN (0,1,2)),
 opening_cents BIGINT NOT NULL DEFAULT 0 CHECK(opening_cents>=0), gift_account INTEGER NOT NULL DEFAULT 0 CHECK(gift_account IN (0,1)),
 last_seen TEXT, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,app_id,uid)
);
CREATE TABLE IF NOT EXISTS earnings (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), account_id BIGINT NOT NULL REFERENCES accounts(id),
 employee_id BIGINT REFERENCES employees(id), team_id BIGINT REFERENCES teams(id), channel_id BIGINT NOT NULL REFERENCES channels(id),
 day TEXT NOT NULL, hour INTEGER NOT NULL DEFAULT 0 CHECK(hour BETWEEN 0 AND 23), amount_cents BIGINT NOT NULL,
 kind TEXT NOT NULL CHECK(kind IN ('income','adjustment')), source_ref TEXT NOT NULL, source TEXT NOT NULL DEFAULT 'manual',
 adjusts_id BIGINT REFERENCES earnings(id), note TEXT NOT NULL DEFAULT '', created_by BIGINT NOT NULL REFERENCES users(id),
 created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), UNIQUE(company_id,source,source_ref)
);
CREATE INDEX IF NOT EXISTS ix_earnings_account ON earnings(company_id,account_id,day);
CREATE INDEX IF NOT EXISTS ix_earnings_day ON earnings(company_id,day,employee_id);
CREATE TABLE IF NOT EXISTS withdrawals (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), account_id BIGINT NOT NULL REFERENCES accounts(id),
 holder_id BIGINT REFERENCES holders(id), holder_channel_id BIGINT REFERENCES holder_channels(id),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>0), fee_cents BIGINT NOT NULL DEFAULT 0 CHECK(fee_cents>=0 AND fee_cents<amount_cents),
 percent_bps INTEGER NOT NULL DEFAULT 0 CHECK(percent_bps BETWEEN 0 AND 10000), holder_share_cents BIGINT NOT NULL DEFAULT 0,
 state TEXT NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','approved','paid','rejected')),
 idempotency_key TEXT NOT NULL, payload_hash TEXT NOT NULL, note TEXT NOT NULL DEFAULT '', review_note TEXT NOT NULL DEFAULT '',
 payment_ref TEXT, created_by BIGINT NOT NULL REFERENCES users(id), reviewed_by BIGINT REFERENCES users(id),
 paid_at TEXT, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), updated_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text),
 version INTEGER NOT NULL DEFAULT 1, UNIQUE(company_id,idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS ix_payment_ref ON withdrawals(company_id,payment_ref) WHERE payment_ref IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_withdraw_account ON withdrawals(company_id,account_id,state);
CREATE TABLE IF NOT EXISTS settlements (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), holder_id BIGINT NOT NULL REFERENCES holders(id),
 amount_cents BIGINT NOT NULL CHECK(amount_cents>=0), state TEXT NOT NULL DEFAULT 'draft' CHECK(state IN ('draft','paid')),
 start_date TEXT NOT NULL, end_date TEXT NOT NULL, idempotency_key TEXT NOT NULL, payload_hash TEXT NOT NULL,
 payment_ref TEXT, created_by BIGINT NOT NULL REFERENCES users(id), paid_by BIGINT REFERENCES users(id), paid_at TEXT,
 created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,idempotency_key)
);
CREATE UNIQUE INDEX IF NOT EXISTS ix_settlement_payment_ref ON settlements(company_id,payment_ref) WHERE payment_ref IS NOT NULL;
CREATE TABLE IF NOT EXISTS settlement_items (
 settlement_id BIGINT NOT NULL REFERENCES settlements(id), withdrawal_id BIGINT NOT NULL UNIQUE REFERENCES withdrawals(id),
 share_cents BIGINT NOT NULL, PRIMARY KEY(settlement_id,withdrawal_id)
);
CREATE TABLE IF NOT EXISTS blacklist (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), app_id BIGINT NOT NULL REFERENCES apps(id),
 uid TEXT NOT NULL, reason TEXT NOT NULL, status INTEGER NOT NULL DEFAULT 0,
 created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1, UNIQUE(company_id,app_id,uid)
);
CREATE TABLE IF NOT EXISTS configs (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), account_id BIGINT NOT NULL REFERENCES accounts(id),
 friends_circle_end_date TEXT, close_friends_end_date TEXT, note TEXT NOT NULL DEFAULT '',
 status INTEGER NOT NULL DEFAULT 0, created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text), version INTEGER NOT NULL DEFAULT 1,
 UNIQUE(company_id,account_id)
);
CREATE TABLE IF NOT EXISTS bans (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), account_id BIGINT NOT NULL REFERENCES accounts(id),
 employee_id BIGINT REFERENCES employees(id), reason TEXT NOT NULL, balance_cents BIGINT NOT NULL,
 created_by BIGINT NOT NULL REFERENCES users(id), created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text)
);
CREATE TABLE IF NOT EXISTS audit_logs (
 id BIGSERIAL PRIMARY KEY, company_id BIGINT NOT NULL REFERENCES companies(id), user_id BIGINT NOT NULL REFERENCES users(id),
 action TEXT NOT NULL, entity TEXT NOT NULL, entity_id BIGINT, detail TEXT NOT NULL,
 created_at TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP::text)
);
CREATE INDEX IF NOT EXISTS ix_audit_company ON audit_logs(company_id,id);
CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
