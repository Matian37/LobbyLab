import postgres from 'postgres';
import { SESSION_TOKEN_LENGTH } from './constants.js';

const DATABASE_URL = process.env.DATABASE_URL;

export const sql = postgres(DATABASE_URL);

export async function getConnectionStatuses(logins) {
    if (logins.length === 0) return new Map();

    const rows = await sql`
        SELECT
            u.login,
            u.last_websocket_id,
            u.match_id::int,
            u.match_auth_token,
            m.host,
            m.port
        FROM users u
        LEFT JOIN matches m ON m.id = u.match_id
        WHERE u.login = ANY(${sql.array(logins)})
    `;
    return new Map(
        rows.map((row) => [
            row.login,
            {
                websocketId: row.last_websocket_id,
                matchId: row.match_id,
                matchAuthToken: row.match_auth_token,
                host: row.host,
                port: row.port,
            },
        ])
    );
}

export async function addUser(login, password) {
    try {
        await sql`
            INSERT INTO users (login, password)
            VALUES(${login}, crypt(${password}, gen_salt('bf')))
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
        SELECT password = crypt(${password}, password) AS match 
        FROM users
        WHERE login = ${login}
    `;
    return q.length > 0 && q[0].match;
}

export async function extendQueueStatuses(connections, ms) {
    if (connections.length === 0) return;

    const rows = sql(
        connections.map(({ login, websocketId }) => [login, websocketId])
    );

    await sql`
        UPDATE users u
        SET queued_until = NOW() + ${ms} * INTERVAL '1 millisecond'
        FROM (VALUES ${rows}) AS v(login, websocket_id)
        WHERE 
            u.login = v.login 
            AND u.last_websocket_id = v.websocket_id 
            AND u.match_id IS NULL
    `;
}

export async function setUserWebsocket(login, websocketId, ms) {
    const q = await sql`
        UPDATE users
        SET last_websocket_id = ${websocketId},
            queued_until = NOW() + ${ms} * INTERVAL '1 millisecond'
        WHERE login = ${login} AND match_id IS NULL
    `;
    return q.count != 0;
}

export async function removeQueueStatus(login, websocketId) {
    await sql`
        UPDATE users
        SET queued_until = NOW() - INTERVAL '1 second',
            last_websocket_id = NULL
        WHERE
            login = ${login}
            AND last_websocket_id = ${websocketId}
            AND match_id IS NULL
    `;
}

export async function isWaiting(login) {
    const q = await sql`
        SELECT 1 FROM users
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
    try {
        const q = await sql`
            INSERT INTO sessions (token, login, date)
            VALUES (encode(gen_random_bytes(${SESSION_TOKEN_LENGTH / 2}), 'hex'), ${login}, NOW())
            RETURNING token
        `;
        return q[0].token;
    } catch (err) {
        if (!(err instanceof postgres.PostgresError)) throw err;
        if (
            err.code !== '23503' ||
            err.constraint_name !== 'sessions_login_fkey'
        )
            throw err;
        return null;
    }
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
