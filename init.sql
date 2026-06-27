CREATE TABLE IF NOT EXISTS matches(
    id SERIAL PRIMARY KEY,
    host TEXT NOT NULL,
    port INT NOT NULL,
    active BOOLEAN NOT NULL DEFAULT true
);

CREATE TABLE IF NOT EXISTS users(
    login TEXT PRIMARY KEY,
    password TEXT NOT NULL,
    match_id INT REFERENCES matches(id)
);

CREATE TABLE IF NOT EXISTS results(
    match_id INT,
    players_count INT,
    players TEXT[]
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

CREATE OR REPLACE FUNCTION notify_waiting()
RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    payload TEXT;
BEGIN
    IF TG_OP = 'INSERT' THEN
        payload := json_build_object(
            'username', NEW.login
        )::TEXT;
    END IF;

    PERFORM pg_notify('new_waiting_user', payload);

    RETURN NULL;
END;
$$;

CREATE OR REPLACE TRIGGER trg_waiting
AFTER UPDATE ON waiting
FOR EACH ROW
EXECUTE FUNCTION notify_waiting();