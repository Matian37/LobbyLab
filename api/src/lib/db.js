import postgres from 'postgres';
import { SESSION_TOKEN_LENGTH } from './constants.js';
import { databaseLogger } from './logger.js';

const DATABASE_URL = process.env.DATABASE_URL;

/**
 * Shared postgres connection pool. Reads the connection string from
 * `DATABASE_URL`. Emits structured log records on connect, close, notice, and
 * every executed query.
 */
export const sql = postgres(DATABASE_URL, {
    onnotice: (notice) => databaseLogger.warn('database notice', notice),
    onconnect: (conn) => databaseLogger.debug('database connected', { conn }),
    onclose: (conn) =>
        databaseLogger.debug('database connection closed', { conn }),
    debug: (conn, query, params) =>
        databaseLogger.debug('database query', { conn, query, params }),
});

/**
 * Connection status snapshot used by the WebSocket pull loop to decide the
 * fate of a queued connection.
 *
 * @typedef {Object} ConnectionStatus
 * @property {number} websocketId The user's current `last_websocket_id`.
 * @property {number|null} matchId ID of the user's assigned match, or `null`.
 * @property {string} matchAuthToken Token to join the match, may be old when
 * the match is not assigned.
 * @property {string} host Game server host of the match or empty string.
 * @property {string} port Game server port of the match or empty string.
 */

/**
 * Fetches the current connection-related statuses for the given connections in
 * a single query, keyed by login.
 *
 * @param {{ login: string }[]} connections Active connections to look up.
 * @returns {Promise<Map<string, ConnectionStatus>>} A map from login to its
 *     status. Users that no longer exist are absent from the map.
 */
export async function getConnectionStatuses(connections) {
    if (connections.length === 0) return new Map();

    const logins = sql.array(connections.map((c) => c.login));

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
        WHERE u.login = ANY(${logins})
    `;
    return new Map(
        rows.map((row) => [
            row.login,
            {
                websocketId: Number(row.last_websocket_id),
                matchId: row.match_id,
                matchAuthToken: row.match_auth_token,
                host: row.host,
                port: row.port,
            },
        ])
    );
}

/**
 * Creates a new user. The password is hashed with bcrypt (`crypt` /
 * `gen_salt('bf')`) by the database.
 *
 * @param {string} login Desired unique login.
 * @param {string} password Plain-text password, hashed by the database.
 * @returns {Promise<boolean>} `true` on success; `false` when the `login` is
 *     already taken (a `users_pkey` uniqueness violation).
 */
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

/**
 * Verifies a login/password pair against the stored bcrypt hash.
 *
 * @param {string} login The user's login.
 * @param {string} password Plain-text password to compare.
 * @returns {Promise<boolean>} `true` when the user exists and the password
 *     matches, otherwise `false`.
 */
export async function verifyPassword(login, password) {
    const q = await sql`
        SELECT password = crypt(${password}, password) AS match 
        FROM users
        WHERE login = ${login}
    `;
    return q.length > 0 && q[0].match;
}

/**
 * Extends the queue deadline (`queued_until`) for connections that are still
 * queued and match the given websocket ID.
 *
 * @param {{ login: string, websocketId: number }[]} connections Connections to
 *     extend. The update only applies if the stored `last_websocket_id` still
 *     equals the connection's `websocketId`, the user is not in a match, and
 *     they are currently queued.
 * @param {number} ms Milliseconds to add to the current deadline.
 * @returns {Promise<void>}
 */
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
            AND u.last_websocket_id = v.websocket_id::bigint 
            AND u.match_id IS NULL
            AND u.queued_until IS NOT NULL
    `;
}

/**
 * Marks a user as queued and allocates them a new websocket ID by
 * incrementing `last_websocket_id`. A fresh ID is what lets newer connections
 * supersede older ones.
 *
 * @param {string} login The user to put into the queue.
 * @param {number} ms Milliseconds to set the initial queue deadline.
 * @returns {Promise<number|null>} The newly allocated websocket ID, or `null`
 *     when the user is already in a match and cannot be queued.
 */
export async function setQueueStatus(login, ms) {
    const q = await sql`
        UPDATE users
        SET last_websocket_id = last_websocket_id + 1,
            queued_until = NOW() + ${ms} * INTERVAL '1 millisecond'
        WHERE login = ${login} AND match_id IS NULL
        RETURNING last_websocket_id
    `;
    return Number(q[0]?.last_websocket_id) || null;
}

/**
 * Clears a user's queue deadline so they are no longer considered queued. The
 * update is applied only when the stored `last_websocket_id` matches and the
 * user is not yet in a match, so a stale connection cannot unqueue a newer one.
 *
 * @param {string} login The user's login.
 * @param {number} websocketId Websocket ID the update must match.
 * @returns {Promise<void>}
 */
export async function removeQueueStatus(login, websocketId) {
    await sql`
        UPDATE users
        SET queued_until = NULL
        WHERE
            login = ${login}
            AND last_websocket_id = ${websocketId}
            AND match_id IS NULL
    `;
}

/**
 * Resolves a session token to the owning user's login.
 *
 * @param {string} token Session token.
 * @returns {Promise<string|null>} The login, or `null` when no session matches
 *     the token.
 */
export async function getLoginFromToken(token) {
    const q = await sql`
        SELECT login FROM sessions WHERE token = ${token}
    `;
    return q[0]?.login ?? null;
}

/**
 * Returns the active match assigned to a user, if any.
 *
 * @param {string} login The user's login.
 * @returns {Promise<{ host: string, port: string, matchAuthToken: string }|null>}
 *     The match connection details, or `null` when the user has no match.
 */
export async function getUserMatch(login) {
    const q = await sql`
        SELECT u.match_auth_token, m.host, m.port
        FROM users u
        LEFT JOIN matches m ON m.id = u.match_id
        WHERE u.login = ${login} AND u.match_id IS NOT NULL
    `;
    if (q.length === 0) return null;

    return {
        host: q[0].host,
        port: q[0].port,
        matchAuthToken: q[0].match_auth_token,
    };
}

/**
 * Creates a new session for a user and returns its token. The token is
 * `SESSION_TOKEN_LENGTH / 2` random bytes encoded as lowercase hex. The login
 * is stored on the session row via the `sessions_login_fkey` foreign key, which
 * also protects against creating a session for a nonexistent user.
 *
 * @param {string} login The user to create the session for.
 * @returns {Promise<string|null>} The session token, or `null` when the user no
 *     longer exists (a `sessions_login_fkey` violation).
 */
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

/**
 * Deletes a session by token. Does nothing when the token does not exist.
 *
 * @param {string} token The session token.
 * @returns {Promise<void>}
 */
export async function deleteSession(token) {
    await sql`DELETE FROM sessions WHERE token = ${token}`;
}

/**
 * Checks whether a session token currently exists.
 *
 * @param {string} token The session token.
 * @returns {Promise<boolean>} `true` when the session exists.
 */
export async function sessionExist(token) {
    const q = await sql`
        SELECT 1 FROM sessions WHERE token = ${token}
    `;
    return q.length > 0;
}

/**
 * Fetches the historical match results a user has ever participated in.
 *
 * @param {string} login The user's login.
 * @returns {Promise<Array<{
 *     id: number,
 *     details: import('postgres').JsonValue|null,
 *     canceled: boolean,
 *     active: boolean
 * }>>} Matches, most recent first.
 */
// TODO: implement pagination
export async function getMatchResults(login) {
    return (
        await sql`
            SELECT m.id, m.results, m.canceled, m.active
            FROM matches m
            WHERE m.id IN (
                SELECT match_id
                FROM user_matches
                WHERE user_id = ${login} 
            )
            ORDER BY m.id DESC
        `
    ).map((row) => ({
        id: Number(row.id),
        details: row.results,
        canceled: row.canceled,
        active: row.active,
    }));
}
