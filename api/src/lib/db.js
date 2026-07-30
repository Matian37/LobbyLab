import postgres from 'postgres';

const DATABASE_URL = process.env.DATABASE_URL;

export const sql = postgres(DATABASE_URL);

export async function addUser(login, password) {
    try {
        await sql`
            INSERT INTO users (login, password)
            VALUES(${login}, crypt(${password}, gen_salt('bf')))
            RETURNING *
        `;
    } catch (err) {
        if (!(err instanceof postgres.PostgresError)) throw err;
        if (err.code !== '23505' || err.constraint_name !== 'users_pkey')
            throw err;
        return false;
    }

    return true;
}

export async function verifyPassword(login, password) {
    const q = await sql`
        SELECT (password = crypt(${password}, password)) AS match 
        FROM users
        WHERE login = ${login}
    `;
    return q.length > 0 && q[0].match;
}

export async function extendQueueStatus(login) {
    await sql`
        UPDATE users
        SET queued_until = NOW() + INTERVAL '5 seconds'
        WHERE login = ${login}
    `;
}

export async function isWaiting(login) {
    const q = await sql`
        SELECT * FROM users
        WHERE
            login = ${login} 
            AND match_id IS NULL 
            AND queued_until > NOW()
    `;
    return q.length > 0;
}

export async function getLoginFromToken(token) {
    const q = await sql`
        SELECT login FROM sessions WHERE token = ${token}
    `;
    return q[0]?.login ?? null;
}

export async function addSession(login) {
    const q = await sql`
        INSERT INTO sessions (token, login, date)
        VALUES (encode(gen_random_bytes(32), 'hex'), ${login}, NOW())
        RETURNING token
    `;
    return q[0].token;
}

export async function deleteSession(token) {
    await sql`DELETE FROM sessions WHERE token = ${token}`;
}

export async function sessionExist(token) {
    const q = await sql`
        SELECT 1 FROM sessions WHERE token = ${token}
    `;
    return q.length > 0;
}

export async function getMatchResults(login) {
    return (
        await sql`
            SELECT m.results, m.canceled
            FROM matches m
            WHERE m.id IN (
                SELECT match_id
                FROM user_matches
                WHERE user_id = ${login} 
            )
        `
    ).map((row) => ({
        details: row.results,
        canceled: row.canceled,
    }));
}

export async function getAuthToken(login) {
    const q = await sql`
        SELECT match_auth_token FROM users WHERE login = ${login}
    `;
    return q[0]?.match_auth_token ?? null;
}
