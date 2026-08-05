CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SEQUENCE matches_id_seq;

CREATE TABLE IF NOT EXISTS matches(
    id SERIAL PRIMARY KEY,
    host TEXT NOT NULL,
    port TEXT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true,
    canceled BOOLEAN NOT NULL DEFAULT false,
    results JSONB
);

ALTER SEQUENCE matches_id_seq OWNED BY matches.id;

CREATE TABLE IF NOT EXISTS users(
    login TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    match_id BIGINT REFERENCES matches(id) ON DELETE SET NULL,
    match_auth_token TEXT,
    queued_until TIMESTAMP,
    last_websocket_id BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_users_match_id ON users(match_id);
CREATE INDEX IF NOT EXISTS idx_users_login_match_id ON users(login, match_id, queued_until);

-- Stores matches in which users have participated
CREATE TABLE IF NOT EXISTS user_matches (
    user_id TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, match_id)
);

-- TODO: cleanup outdated sessions
CREATE TABLE IF NOT EXISTS sessions(
    token TEXT PRIMARY KEY,
    login TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    date DATE
);
