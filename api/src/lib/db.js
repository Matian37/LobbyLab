import Database from 'better-sqlite3';
import { handleError } from './error_handler';
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
    login TEXT NOT NULL,
    date DATE
);
`);

export function findUserByLogin(login){
    const q = db.prepare(`SELECT * FROM users
        WHERE login = ?
    `);
    return q.get(login);
}

export function addUser(login, password){
    try{
        const q = db.prepare('INSERT INTO users (login, password) VALUES(?, ?)');
        q.run(login, password);
        return true;
    }
    catch (err){
        handleError(-1, err);
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
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export function deleteFromWaiting(login){
    try{
        const q = db.prepare('DELETE FROM waiting WHERE login=?');
        q.run(login);
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export function getLoginFromToken(token){
    const q = db.prepare('SELECT login FROM sessions WHERE token = ?');
    return q.get(token).login;
}

export function setSession(token, login){
    try{
        const q = db.prepare('INSERT INTO sessions (token, login, date) VALUES (?, ?, CURRENT_TIMESTAMP)');
        q.run(token, login);
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export function deleteSession(token){
    try{
        const q = db.prepare('DELETE FROM sessions WHERE token = ?');
        q.run(token);
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export function tokenExists(token){
    const q = db.prepare('SELECT * FROM sessions WHERE token = ?');
    return q.get(token);
}

function deleteOldSessions(){
    const q = db.prepare(`DELETE FROM sessions WHERE date < datetime('now', '-3 months')`);
    q.run();
}

setInterval(deleteOldSessions, 1000 * 60 * 60 * 24);