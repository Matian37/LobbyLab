import Database from 'better-sqlite3';
const db = new Database('../base.db');

db.exec(`CREATE TABLE IF NOT EXISTS users(
    login TEXT PRIMARY KEY,
    password TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS waiting(
    login TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS sessions(
    token TEXT PRIMARY KEY,
    login TEXT NOT NULL
);
`);

export function findUserByLogin(login){
    const q = db.prepare(`SELECT * FROM users
        WHERE login = ?
    `);
    return q.get(login);
}

export function addUser(login, password){
    console.debug(login + " elo " + password);
    try{
        const q = db.prepare('INSERT INTO users (login, password) VALUES(?, ?)');
        q.run(login, password);
        console.debug("true");
        return true;
    }
    catch (err){
        console.debug(err + "<-blad");
        return false;
    }
}

export function findWaitingByLogin(login){
    const q = db.prepare(`SELECT * FROM waiting WHERE login = ?`);
    return q.get(login);
}

export function addToWaiting(login){
    try{
        const q = db.prepare(`INSERT INTO waiting (login) VALUES(?)`);
        q.run(login);
        return true;
    }
    catch{
        return false;
    }
}

export function deleteFromWaiting(login){
    try{
        const q = db.prepare('DELETE FROM waiting WHERE login=?');
        q.run(login);
        return true;
    }
    catch{
        return false;
    }
}

export function getLoginFromToken(token){
    const q = db.prepare('SELECT login FROM sessions WHERE token = ?');
    return q.get(token).login;
}

export function setSession(token, login){
    try{
        const q = db.prepare('INSERT INTO sessions (token, login) VALUES (?, ?)');
        q.run(token, login);
        return true;
    }
    catch{
        return false;
    }
}

export function deleteSession(token){
    try{
        const q = db.prepare('DELETE FROM sessions WHERE token = ?');
        q.run(token);
        return true;
    }
    catch{
        return false;
    }
}

export function tokenExists(token){
    const q = db.prepare('SELECT * FROM sessions WHERE token = ?');
    return q.get(token);
}