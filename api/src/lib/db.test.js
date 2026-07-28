import {
    expect,
    it,
    beforeEach,
    afterEach,
    beforeAll,
    afterAll,
    describe,
    vi,
} from 'vitest';
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

describe('tryQuery', () => {
    beforeEach(() => {
        vi.spyOn(console, 'debug').mockImplementation(() => {});
    });
    afterEach(() => {
        vi.restoreAllMocks();
    });

    it('returns false and logs when the callback throws', async () => {
        const result = await db.tryQuery(async () => {
            throw new Error('Test Error Message');
        });
        expect(result).toBe(false);
        expect(console.debug).toHaveBeenCalled();
        expect(console.debug).toHaveBeenCalledWith(
            expect.objectContaining({ message: 'Test Error Message' })
        );
    });

    it('returns true when the callback succeeds', async () => {
        const result = await db.tryQuery(async () => {});
        expect(result).toBe(true);
    });
});

describe('addUser', () => {
    it('inserts user into the database', async () => {
        await db.addUser('alice', 'secret');
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
    it('sets user status to waiting in queue', async () => {
        expect(await db.addUser('alice', 'secret')).toBe(true);
        expect(await db.extendQueueStatus('alice')).toBe(true);
        const now = Date.now();
        const rows = await helperSql`
            SELECT queued_until > NOW() as cond FROM users
        `;
        expect(rows.length).toBe(1);
        expect(rows[0].cond).toBe(true);
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

    it('returns true even when the token does not exist', async () => {
        expect(await db.deleteSession('nonexistent')).toBe(true);
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
