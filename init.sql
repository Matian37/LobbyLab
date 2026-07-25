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
    match_id BIGINT REFERENCES matches(id) ON DELETE SET NULL
);
CREATE INDEX IF NOT EXISTS idx_users_match_id ON users(match_id);

-- Stores matches in which users have participated
CREATE TABLE IF NOT EXISTS user_matches (
    user_id TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    match_id BIGINT NOT NULL REFERENCES matches(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, match_id)
);

CREATE TABLE IF NOT EXISTS waiting(
    login TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS sessions(
    token TEXT PRIMARY KEY,
    login TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    date DATE
);


CREATE OR REPLACE FUNCTION notify_users_match_id()
RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    payload TEXT;
BEGIN
    SELECT json_build_object(
        'username', NEW.login,
        'host', m.host,
        'port', m.port
    )::TEXT
    INTO payload
    FROM matches m
    WHERE m.id = NEW.match_id;
    PERFORM pg_notify('users_match_id_assigned', payload);

    RETURN NEW;
END;
$$;

CREATE OR REPLACE TRIGGER trg_users_after_update
AFTER UPDATE ON users
FOR EACH ROW
WHEN (OLD.match_id IS NULL AND NEW.match_id IS NOT NULL)
EXECUTE FUNCTION notify_users_match_id();
