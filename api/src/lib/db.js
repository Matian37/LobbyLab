import postgres from "postgres";
import { handleError } from "./error_handler";
let DATABASE_URL = process.env.DATABASE_URL;
if(process.env.VITEST)
    DATABASE_URL = 'postgresql://postgres:123@localhost:5432/postgres';

const sql = postgres(DATABASE_URL);
console.debug(DATABASE_URL + "<-- moj link do bazy");

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

export async function findWaitingByLogin(login){
    const q = await sql`
        SELECT * FROM waiting WHERE login = ${login}
    `
    return q;
}

export async function addToWaiting(login){
    try{
        await sql`
            INSERT INTO waiting (login) VALUES(${login})
        `
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
}

export async function deleteFromWaiting(login){
    try{
        await sql`
            DELETE FROM waiting WHERE login=${login}
        `
        return true;
    }
    catch (err){
        handleError(-1, err);
        return false;
    }
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

setInterval(deleteOldSessions, 1000 * 60 * 60 * 24);

export async function truncateEverything(){
    await sql`
        TRUNCATE TABLE users RESTART IDENTITY CASCADE;
    `
    await sql`
        TRUNCATE TABLE waiting RESTART IDENTITY CASCADE;
    `
    await sql`
        TRUNCATE TABLE sessions RESTART IDENTITY CASCADE;
    `
}