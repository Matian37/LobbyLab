CREATE TABLE IF NOT EXISTS matches(
    id SERIAL PRIMARY KEY,
    host TEXT NOT NULL,
    port INT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS users(
    login TEXT PRIMARY KEY,
    password BYTEA NOT NULL,
    match_id INT REFERENCES matches(id)
);

CREATE TABLE IF NOT EXISTS sessions(
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE
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
