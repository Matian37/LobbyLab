import postgres from "postgres";

let DATABASE_URL = process.env.DATABASE_URL;

export const sql = postgres(DATABASE_URL);

export async function findUserByLogin(login){
    const q = await sql`
        SELECT * FROM users WHERE login = ${login}
    `
    return q[0] ?? null;
}

export async function addUser(login, password){
    try{
        const result = await sql`
            INSERT INTO users (login, password) VALUES(${login}, ${password}) RETURNING *
        `
        return true;
    }
    catch (err){
        console.debug(err);
        return false;
    }
}

export async function isWaiting(login){
    const q = await sql`
        SELECT 1 FROM waiting WHERE login = ${login}
    `
    return q.length > 0;
}

export async function addToWaiting(login){
    try{
        await sql`
            INSERT INTO waiting (login) VALUES(${login})
        `
        return true;
    }
    catch (err){
        console.debug(err);
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
        console.debug(err);
        return false;
    }
}

export async function getLoginFromToken(token){
    const q = await sql`
        SELECT login FROM sessions WHERE token = ${token}
    `
    return q[0]?.login ?? null;
}

export async function addSession(login){
    const q = await sql`
        INSERT INTO sessions (token, login, date)
        VALUES (encode(gen_random_bytes(32), 'hex'), ${login}, NOW())
        RETURNING token
    `
    return q[0].token;
}

export async function deleteSession(token){
    try{
        await sql`
            DELETE FROM sessions WHERE token = ${token}
        `
        return true;
    }
    catch (err){
        console.debug(err);
        return false;
    }
}

export async function tokenExists(token){
    const q = await sql`
        SELECT 1 FROM sessions WHERE token = ${token}
    `
    return q.length > 0;
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
