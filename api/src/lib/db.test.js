import { expect, it, beforeEach, beforeAll, afterAll, describe } from 'vitest';
import { PostgreSqlContainer } from '@testcontainers/postgresql';
import postgres from 'postgres';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

let db, helperSql, container;

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

    const pg = postgres(databaseUrl, { onnotice: () => {} });
    await pg.unsafe(initSql);
    await pg.end();
    console.log('[test-db] schema initialized');

    console.log('[test-db] importing $lib/db.js...');
    db = await import('$lib/db.js');
    helperSql = postgres(databaseUrl, { onnotice: () => {} });
    console.log('[test-db] $lib/db.js loaded, connection pool ready');
}, 30000);

beforeEach(async () => {
    await helperSql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await helperSql.unsafe(process.env.DATABASE_INIT_SQL);
});

afterAll(async () => {
    if (helperSql) await helperSql.end();

    if (container) {
        console.log('[test-db] stopping...');
        await container.stop();
        console.log('[test-db] stopped');
    }
});

describe('addUser', () => {
    it('inserts user into the database and returns true', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        const rows = await helperSql`
            SELECT login, password = crypt('pass', password) AS match
            FROM users
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].login).toBe('user');
        expect(rows[0].match).toBeTruthy();
    });

    it('returns false on duplicate login', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        expect(await db.addUser('user', 'pass')).toBeFalsy();
    });

    it('re-throws non-PostgresError', async () => {
        await expect(db.addUser(undefined, 'pass')).rejects.toMatchObject({
            code: 'UNDEFINED_VALUE',
        });
    });

    it('re-throws PostgresError with different code', async () => {
        await expect(db.addUser('user', null)).rejects.toMatchObject({
            code: '23502',
        });
    });

    it('re-throws PostgresError with same code but different constraint', async () => {
        await helperSql`ALTER TABLE users ADD COLUMN x TEXT UNIQUE DEFAULT 'same'`;
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        await expect(db.addUser('user2', 'pass')).rejects.toMatchObject({
            code: '23505',
            constraint_name: 'users_x_key',
        });
    });
});

describe('verifyPassword', () => {
    it('returns true when the password matches', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        expect(await db.verifyPassword('user', 'pass')).toBeTruthy();
    });

    it('returns false when the password does not match', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        expect(await db.verifyPassword('user', '')).toBeFalsy();
    });

    it('returns false when the user does not exist', async () => {
        expect(await db.verifyPassword('user', 'pass')).toBeFalsy();
    });
});

describe('extendQueueStatus', () => {
    it('returns true and extends user queue status', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        expect(await db.extendQueueStatus('user')).toBeTruthy();
        const rows = await helperSql`
            SELECT queued_until > NOW() as cond FROM users
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].cond).toBeTruthy();
    });

    it('return false when user does not exist', async () => {
        expect(await db.extendQueueStatus('user')).toBeFalsy();
    });
});

describe('isWaiting', () => {
    it('returns true when user is queued and not matched', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        expect(await db.extendQueueStatus('user')).toBeTruthy();
        expect(await db.isWaiting('user')).toBeTruthy();
    });

    it('returns false when user has an active match_id', async () => {
        await helperSql`
            INSERT INTO matches (id, host, port) VALUES (1, '', '0');
        `;
        await helperSql`
            INSERT INTO users (login, password, match_id, queued_until)
            VALUES ('user', '', 1, NOW() + INTERVAL '5 hours');
        `;
        expect(await db.isWaiting('user')).toBeFalsy();
    });

    it('returns false when queued_until is in the past', async () => {
        await helperSql`
            INSERT INTO users (login, password, match_id, queued_until)
            VALUES ('user', '', NULL, NOW() - INTERVAL '5 hours')
        `;
        expect(await db.isWaiting('user')).toBeFalsy();
    });

    it('returns false when user does not exist', async () => {
        expect(await db.isWaiting('user')).toBeFalsy();
    });
});

describe('addSession', () => {
    it('creates session and stores it', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();

        const token = await db.addSession('user');
        expect(token).toBeTypeOf('string');
        expect(token).toMatch(/^[0-9a-f]+$/);
        expect(token.length).toBe(SESSION_TOKEN_LENGTH);

        const rows = await helperSql`
            SELECT login
            FROM sessions
            WHERE token = ${token}
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].login).toBe('user');
    });

    it('returns null when user does not exist', async () => {
        expect(await db.addSession('user')).toBe(null);
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
        expect(await db.addUser('user', '')).toBeTruthy();
        await expect(db.addSession('user')).rejects.toMatchObject({
            code: '23503',
            constraint_name: 'sessions_x_fkey',
        });
    });
});

describe('getLoginFromToken', () => {
    it('returns login for a valid token', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        const token = await db.addSession('user');
        expect(await db.getLoginFromToken(token)).toBe('user');
    });

    it('returns null when token does not exist', async () => {
        expect(await db.getLoginFromToken('user')).toBe(null);
    });
});

describe('deleteSession', () => {
    it('removes the session from the database', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        const token = await db.addSession('user');

        expect(await db.deleteSession(token)).toBeUndefined();
        const rows = await helperSql`SELECT login FROM sessions`;
        expect(rows.length).toBe(0);
    });

    it('resolves even when the token does not exist', async () => {
        expect(await db.deleteSession('user')).toBeUndefined();
    });
});

describe('sessionExist', () => {
    it('returns true for an existing token', async () => {
        expect(await db.addUser('user', 'pass')).toBeTruthy();
        const token = await db.addSession('user');
        expect(await db.sessionExist(token)).toBeTruthy();
    });

    it('returns false for a non-existent token', async () => {
        expect(await db.sessionExist('token')).toBeFalsy();
    });
});

describe('getMatchResults', () => {
    it.each([
        {
            name: 'returns match for user in its players',
            matches: [
                {
                    players: ['user1', 'user6'],
                    winner: 'user6',
                    canceled: false,
                },
            ],
            expected: [
                {
                    details: {
                        players: ['user1', 'user6'],
                        winner: 'user6',
                    },
                    canceled: false,
                },
            ],
        },
        {
            name: 'returns empty array for user not in its players',
            matches: [
                {
                    players: ['user5', 'user6'],
                    winner: 'user5',
                    canceled: false,
                },
            ],
            expected: [],
        },
        {
            name: 'returns multiple matches for existing user',
            matches: [
                {
                    players: ['user1', 'user6'],
                    winner: 'user1',
                    canceled: false,
                },
                {
                    players: ['user5', 'user1'],
                    winner: 'user5',
                    canceled: true,
                },
                {
                    players: ['user3', 'user4'],
                    winner: 'user3',
                    canceled: false,
                },
            ],
            expected: [
                {
                    details: {
                        players: ['user1', 'user6'],
                        winner: 'user1',
                    },
                    canceled: false,
                },
                {
                    details: {
                        players: ['user5', 'user1'],
                        winner: 'user5',
                    },
                    canceled: true,
                },
            ],
        },
    ])('$name', async ({ matches, expected }) => {
        const createdPlayers = new Set();
        for (const match of matches) {
            for (const player of match.players) {
                if (createdPlayers.has(player)) continue;

                createdPlayers.add(player);

                expect(await db.addUser(player, 'pass')).toBeTruthy();
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

        expect(await db.getMatchResults('user1')).toEqual(expected);
    });

    it('returns an empty array when the user does not exist', async () => {
        expect(await db.getMatchResults('user')).toEqual([]);
    });
});

describe('getAuthToken', () => {
    it('returns auth token for valid login', async () => {
        await helperSql`
            INSERT INTO users (login, password, match_auth_token)
            VALUES ('user', '123', 'token')
        `;
        expect(await db.getAuthToken('user')).toBe('token');
    });

    it('returns null for non-existent login', async () => {
        expect(await db.getAuthToken('user')).toBeNull();
    });

    it('returns null for no token', async () => {
        expect(await db.addUser('login', 'pass')).toBeTruthy();
        expect(await db.getAuthToken('user')).toBeNull();
    });
});
