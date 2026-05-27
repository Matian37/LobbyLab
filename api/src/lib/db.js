import Database from 'better-sqlite3';

const db = new Database('base.db');

db.exec(`
    CREATE TABLE IF NOT EXISTS waiting (
        id INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE,
        login TEXT UNIQUE
    )
`);

db.exec(`
    CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT UNIQUE,
        login TEXT UNIQUE,
        password TEXT
    )
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