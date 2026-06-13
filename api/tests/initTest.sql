CREATE TABLE IF NOT EXISTS users(
    login TEXT PRIMARY KEY,
    password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS waiting(
    login TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS sessions(
    token TEXT PRIMARY KEY,
    login TEXT NOT NULL REFERENCES users(login) ON DELETE CASCADE,
    date DATE
);
