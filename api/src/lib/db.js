import postgres from "postgres";
import { handleError } from "./error_handler";

let DATABASE_URL = process.env.DATABASE_URL;
//if(!DATABASE_URL && process.env.VITEST)
//let DATABASE_URL = 'postgresql://postgres:123@localhost:5432/postgres';

export const sql = postgres(DATABASE_URL);

export async function findUserByLogin(login){
    const q = await sql`
        SELECT * FROM users WHERE login = ${login}
    `
    return q;
}

export async function addUser(login, password){
    try{
        const result = await sql`
            INSERT INTO users (login, password) VALUES(${login}, ${password}) RETURNING *
        `
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export async function setUserStatus(login){
    try{
        await sql`
            UPDATE users SET queued_until = NOW() + INTERVAL '5 seconds' WHERE login = ${login}
        `
        return true;
    }
    catch{
        return false;
    }
}

export async function findWaitingByLogin(login){
    const q = await sql`
        SELECT * FROM users WHERE login = ${login} AND queued_until > NOW()
    `
    return q;
}

export async function getLoginFromToken(token){
    const q = await sql`
        SELECT login FROM sessions WHERE token = ${token}
    `
    return q;
}

export async function setSession(token, login){
    try{
        await sql`
            INSERT INTO sessions (token, login, date) VALUES (${token}, ${login}, CURRENT_TIMESTAMP)
        `
        return true;
    }
    catch (err){
        console.debug(err);
        handleError(-1, err);
        return false;
    }
}

export async function deleteSession(token){
    try{
        await sql`
            DELETE FROM sessions WHERE token = ${token}
        `
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export async function tokenExists(token){
    const q = await sql`
        SELECT * FROM sessions WHERE token = ${token}
    `
    return q;
}

export async function getMatchResults(login){
    const q = await sql`
        SELECT match_id FROM user_matches WHERE user_id = ${login}
    `
    const matchIds = q.map(row => row.match_id);

    const q1 = await sql`
        SELECT results, canceled FROM matches WHERE id = ANY(${matchIds}::int[])
    `
    return q1.map(row => ({
        details: row.results,
        canceled: row.canceled,
    }));
}


async function deleteOldSessions(){
    await sql`
        DELETE FROM sessions WHERE date < NOW() - INTERVAL '3 months'
    `
}

export async function healthCheck(){
    try{
        await sql`SELECT 1`;
        return true;
    }
    catch(err){
        handleError("ERROR " + err);
        return false;
    }
}
