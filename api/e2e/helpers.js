import { readFileSync } from 'node:fs';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const INIT_SQL = readFileSync(
    path.resolve(import.meta.dirname, '../../init.sql'),
    'utf8'
);

export const BASE_URL = 'http://localhost:4173';
export const DB_PORT = 5432;
export const DATABASE_URL = `postgres://postgres:123@localhost:${DB_PORT}/postgres`;
export const TMP_DIR = path.join(os.tmpdir(), 'multiplayer-asset-e2e');
export const DOWNLOADS_DIR = path.join(TMP_DIR, 'downloads');
export const CLIENT_FILE = 'game-client.zip';
export const CLIENT_CONTENT = 'e2e game client archive';

export async function addClientFile() {
    await fs.mkdir(DOWNLOADS_DIR, { recursive: true });
    await fs.writeFile(path.join(DOWNLOADS_DIR, CLIENT_FILE), CLIENT_CONTENT);
}

export async function removeClientFile() {
    await fs.rm(path.join(DOWNLOADS_DIR, CLIENT_FILE), { force: true });
}

export async function resetDb(sql) {
    await sql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await sql.unsafe(INIT_SQL);
}

export async function registerViaUi(page, login, password = 'password') {
    await page.goto(`${BASE_URL}/register`);
    await page.getByTestId('login-input').fill(login);
    await page.getByTestId('password-input').fill(password);
    await page.getByTestId('register-apply').click();
    await page.waitForURL(`${BASE_URL}/`);
}

export async function loginViaUi(page, login, password = 'password') {
    await page.goto(`${BASE_URL}/login`);
    await page.getByTestId('login-input').fill(login);
    await page.getByTestId('password-input').fill(password);
    await page.getByTestId('login-apply').click();
    await page.waitForURL(`${BASE_URL}/`);
}

// --- server-manager simulation (mirrors server-manager/adapters/db.go) ---

export async function generateAuthTokens(sql, logins) {
    await sql`
        UPDATE users
        SET match_auth_token = encode(gen_random_bytes(32), 'base64')
        WHERE login IN ${sql(logins)}
    `;
}

export async function getNextMatchId(sql) {
    const rows = await sql`SELECT nextval('matches_id_seq') AS id`;
    return Number(rows[0].id);
}

export async function addMatch(sql, matchId, logins, { host, port }) {
    await sql.begin(async (tx) => {
        await tx`
            INSERT INTO matches (id, host, port) VALUES (${matchId}, ${host}, ${port})
        `;
        await tx`
            INSERT INTO user_matches (user_id, match_id)
            SELECT login, ${matchId}
            FROM users
            WHERE login IN ${sql(logins)}
        `;
        await tx`
            UPDATE users
            SET match_id = ${matchId}, queued_until = NULL
            WHERE login IN ${sql(logins)}
        `;
    });
}

export async function assignMatch(
    sql,
    logins,
    { host = 'game-host', port = '7777' } = {}
) {
    await generateAuthTokens(sql, logins);
    const matchId = await getNextMatchId(sql);
    await addMatch(sql, matchId, logins, { host, port });
    return { matchId, host, port };
}

export async function saveMatchResults(
    sql,
    matchId,
    results,
    canceled = false
) {
    await sql`
        UPDATE matches
        SET results = ${sql.json(results)}, 
            canceled = ${canceled}, 
            active = false
        WHERE id = ${matchId}
    `;
}

export async function createMatchResult(
    sql,
    logins,
    {
        host = 'game-host',
        port = '7777',
        players,
        winner,
        canceled = false,
    } = {}
) {
    const { matchId } = await assignMatch(sql, logins, { host, port });
    if (canceled) {
        await saveMatchResults(sql, matchId, null, true);
    } else if (players) {
        await saveMatchResults(sql, matchId, { players, winner }, false);
    }
    return matchId;
}
