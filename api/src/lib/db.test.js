import { expect, it, beforeEach, beforeAll, afterAll, describe } from 'vitest';
import { PostgreSqlContainer } from '@testcontainers/postgresql';
import postgres from 'postgres';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

let db;
let helperSql;
let container;

beforeAll(async () => {
    console.log('[test-db] starting PostgreSQL...');
    container = await new PostgreSqlContainer('postgres:18.4-alpine')
        .withUsername('postgres')
        .withPassword('123')
        .withDatabase('postgres')
        .start();

    const databaseUrl = container.getConnectionUri();
    process.env.DATABASE_URL = databaseUrl;
    console.log('[test-db] started at', databaseUrl);

    console.log('[test-db] initializing schema...');
    const initSqlPath = path.resolve(__dirname, '../../../init.sql');
    const initSql = fs.readFileSync(initSqlPath, 'utf8');
    process.env.DATABASE_INIT_SQL = initSql;

    const pg = postgres(databaseUrl);
    await pg.unsafe(initSql);
    await pg.end();
    console.log('[test-db] schema initialized');

    console.log('[test-db] importing $lib/db.js...');
    db = await import('$lib/db.js');
    helperSql = postgres(databaseUrl);
    console.log('[test-db] $lib/db.js loaded, connection pool ready');
}, 30000);

afterAll(async () => {
    if (helperSql) await helperSql.end();

    if (container) {
        console.log('[test-db] stopping...');
        await container.stop();
        console.log('[test-db] stopped');
    }
});

beforeEach(async () => {
    await helperSql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await helperSql.unsafe(process.env.DATABASE_INIT_SQL);
});

describe('addUser', () => {
    it('inserts user into the database and returns true', async () => {
        const result = await db.addUser('alice', 'secret');
        expect(result).toBe(true);
        const rows = await helperSql`
            SELECT login, password = crypt('secret', password) AS match
            FROM users
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].login).toBe('alice');
        expect(rows[0].match).toBe(true);
    });

    it('returns false on duplicate login', async () => {
        await db.addUser('alice', 'secret');
        const result = await db.addUser('alice', 'other');
        expect(result).toBe(false);
    });

    it('re-throws non-PostgresError', async () => {
        await expect(db.addUser(undefined, 'secret')).rejects.toMatchObject({
            code: 'UNDEFINED_VALUE',
        });
    });

    it('re-throws PostgresError with different code', async () => {
        await expect(db.addUser('alice', null)).rejects.toMatchObject({
            code: '23502',
        });
    });

    it('re-throws PostgresError with same code but different constraint', async () => {
        await helperSql`ALTER TABLE users ADD COLUMN x TEXT UNIQUE DEFAULT 'same'`;
        await db.addUser('alice', 'secret');
        await expect(db.addUser('bob', 'secret')).rejects.toMatchObject({
            code: '23505',
            constraint_name: 'users_x_key',
        });
    });
});

describe('verifyPassword', () => {
    it('returns true when the password matches', async () => {
        await db.addUser('alice', 'correct');
        expect(await db.verifyPassword('alice', 'correct')).toBe(true);
    });

    it('returns false when the password does not match', async () => {
        await db.addUser('alice', 'correct');
        expect(await db.verifyPassword('alice', 'wrong')).toBe(false);
    });

    it('returns false when the user does not exist', async () => {
        expect(await db.verifyPassword('nobody', 'anything')).toBe(false);
    });
});

describe('extendQueueStatus', () => {
    it('returns true and extends user queue status', async () => {
        expect(await db.addUser('alice', 'secret')).toBe(true);
        expect(await db.extendQueueStatus('alice')).toBe(true);
        const rows = await helperSql`
            SELECT queued_until > NOW() as cond FROM users
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].cond).toBe(true);
    });

    it('return false when user does not exist', async () => {
        expect(await db.extendQueueStatus('nonexistent')).toBe(false);
    });
});

describe('isWaiting', () => {
    it('returns true when user is queued and not matched', async () => {
        await db.addUser('alice', 'secret');
        await db.extendQueueStatus('alice');
        expect(await db.isWaiting('alice')).toBe(true);
    });

    it('returns false when user has an active match_id', async () => {
        await helperSql`
            INSERT INTO matches (id, host, port) VALUES (1, '', '0')
        `;
        await helperSql`
            INSERT INTO users (login, password, match_id, queued_until)
            VALUES ('alice', '', 1, NOW() + INTERVAL '5 hours')
        `;
        expect(await db.isWaiting('alice')).toBe(false);
    });

    it('returns false when queued_until has expired', async () => {
        await helperSql`
            INSERT INTO users (login, password, match_id, queued_until)
            VALUES ('alice', '', NULL, NOW() - INTERVAL '5 hours')
        `;
        expect(await db.isWaiting('alice')).toBe(false);
    });

    it('returns false when user does not exist', async () => {
        expect(await db.isWaiting('nobody')).toBe(false);
    });
});

describe('addSession', () => {
    it('creates session and stores it', async () => {
        await db.addUser('alice', 'secret');
        const token = await db.addSession('alice');
        expect(token).toBeTypeOf('string');
        expect(token).toMatch(/^[0-9a-f]+$/);
        expect(token.length).toBeGreaterThanOrEqual(32);

        const rows = await helperSql`
            SELECT login
            FROM sessions
            WHERE token = ${token}
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].login).toBe('alice');
    });

    it('returns null when user does not exist', async () => {
        const token = await db.addSession('nonexistent');
        expect(token).toBe(null);
    });

    it('re-throws non-PostgresError', async () => {
        await expect(db.addSession(undefined)).rejects.toMatchObject({
            code: 'UNDEFINED_VALUE',
        });
    });

    it('re-throws PostgresError with different code', async () => {
        await expect(db.addSession(null)).rejects.toMatchObject({
            code: '23502',
        });
    });

    it('re-throws PostgresError with same code but different constraint', async () => {
        await helperSql`
            ALTER TABLE sessions ADD COLUMN x TEXT REFERENCES users(login) DEFAULT ''
        `;
        expect(await db.addUser('alice', '')).toBe(true);
        await expect(db.addSession('alice')).rejects.toMatchObject({
            code: '23503',
            constraint_name: 'sessions_x_fkey',
        });
    });
});

describe('getLoginFromToken', () => {
    it('returns the login for a valid token', async () => {
        await db.addUser('alice', 'secret');
        const token = await db.addSession('alice');
        expect(await db.getLoginFromToken(token)).toBe('alice');
    });

    it('returns null for a non-existent token', async () => {
        expect(await db.getLoginFromToken('nonexistent')).toBe(null);
    });
});

describe('deleteSession', () => {
    it('removes the session from the database', async () => {
        await db.addUser('alice', 'secret');
        const token = await db.addSession('alice');
        await db.deleteSession(token);
        const rows = await helperSql`
            SELECT login
            FROM sessions
        `;
        expect(rows.length).toBe(0);
    });

    it('resolves even when the token does not exist', async () => {
        await db.deleteSession('nonexistent');
    });
});

describe('sessionExist', () => {
    it('returns true for an existing token', async () => {
        await db.addUser('alice', 'secret');
        const token = await db.addSession('alice');
        expect(await db.sessionExist(token)).toBe(true);
    });

    it('returns false for a non-existent token', async () => {
        expect(await db.sessionExist('nonexistent')).toBe(false);
    });
});

describe('getMatchResults', () => {
    it('returns match results for an existing user', async () => {
        const matches = [
            { players: ['user5', 'user6'], winner: 'user5', canceled: false },
            { players: ['user1', 'user5'], winner: 'user1', canceled: true },
            { players: ['user3', 'user4'], winner: 'user3', canceled: false },
        ];

        const created = new Set();
        for (const match of matches) {
            for (const player of match.players) {
                if (created.has(player)) continue;

                created.add(player);

                await helperSql`
                    INSERT INTO users (login, password)
                    VALUES (${player}, '')
                `;
            }
        }

        for (const match of matches) {
            const { canceled, ...results } = match;

            const match_id = (
                await helperSql`
                    INSERT INTO matches (host, port, results, canceled)
                    VALUES ('', 0, ${helperSql.json(results)}, ${canceled})
                    RETURNING id
                `
            )[0].id;

            for (const player of match.players) {
                await helperSql`
                    INSERT INTO user_matches (user_id, match_id)
                    VALUES (${player}, ${match_id})
                `;
            }
        }

        const result = await db.getMatchResults('user5');
        expect(result).toEqual([
            {
                details: { players: ['user5', 'user6'], winner: 'user5' },
                canceled: false,
            },
            {
                details: { players: ['user1', 'user5'], winner: 'user1' },
                canceled: true,
            },
        ]);
    });

    it('returns an empty array when the user has no matches', async () => {
        await helperSql`
            INSERT INTO users (login, password)
            VALUES (${'lonely'}, '')
        `;
        expect(await db.getMatchResults('lonely')).toEqual([]);
    });
});

describe('getAuthToken', () => {
    it('returns auth token for valid login', async () => {
        await helperSql`INSERT INTO users (login, password, match_auth_token) VALUES ('user', '123', 'token')`;
        const token = await db.getAuthToken('user');
        expect(token).toBe('token');
    });

    it('returns null for invalid login', async () => {
        const token = await db.getAuthToken('invalid');
        expect(token).toBeNull();
    });

    it('returns null for no token', async () => {
        await helperSql`INSERT INTO users (login, password) VALUES ('user', '123')`;
        const token = await db.getAuthToken('user');
        expect(token).toBeNull();
    });
});
