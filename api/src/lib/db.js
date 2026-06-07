import postgres from "postgres";

const sql = postgres(process.env.DATABASE_URL);

export async function findUserByLogin(login){
    const q = await sql`
        SELECT * FROM users WHERE login = ${login}
    `
    return q;
}

export async function addUser(login, password){
    try{
        await sql`
            INSERT INTO users (login, password) VALUES(${login}, ${password})
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

setInterval(deleteOldSessions, 1000 * 60 * 60 * 24);