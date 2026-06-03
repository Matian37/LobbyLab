import Database from 'better-sqlite3';
const db = new Database('../base.db');

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
    catch{
        return false;
    }
}

export function findWaitingByLogin(login){
    const q = db.prepare(`SELECT * FROM waiting WHERE login = ?`);
    return q.get(login);
}

export function addToWaiting(login){
    const q = db.prepare(`INSERT OR IGNORE INTO waiting (login) VALUES(?)`);
    q.run(login);
}

export function deleteFromWaiting(login){
    const q = db.prepare('DELETE FROM waiting WHERE login=?');
    q.run(login);
}

export function getLoginFromToken(token){
    const q = db.prepare('SELECT login FROM sessions WHERE token = ?');
    return q.get(token);
}

export function setSession(token, login){
    const q = db.prepare('INSERT INTO sessions (token, login) VALUES (?, ?)');
    q.run(token, login);
}

export function deleteSession(token){
    const q = db.prepare('DELETE FROM sessions WHERE token = ?');
    q.run(token);
}

export function tokenExists(token){
    const q = db.prepare('SELECT * FROM sessions WHERE token = ?');
    return q.get(token);
}