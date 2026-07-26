import {
    expect,
    test,
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

    test('failure', async () => {
        const result = await db.tryQuery(async () => {
            throw new Error('Test Error Message');
        });
        expect(result).toBe(false);
        expect(console.debug).toHaveBeenCalled();
        expect(console.debug).toHaveBeenCalledWith(
            expect.objectContaining({ message: 'Test Error Message' })
        );
    });

    test('success', async () => {
        const result = await db.tryQuery(async () => {});
        expect(result).toBe(true);
    });
});

describe('user adding and getting', () => {
    test('normal user adding', async () => {
        expect(await db.addUser('user', 'hashedPassword')).toBe(true);
        const q = await helperSql`SELECT 1 FROM users WHERE login = 'user'`;
        expect(q.length).toBe(1);
    });

    test('user adding twice', async () => {
        expect(await db.addUser('user', 'hashedPassword')).toBe(true);
        expect(await db.addUser('user', 'elo')).toBe(false);
    });
});

describe('waiting list', () => {
    test('adding to waiting once', async () => {
        expect(await db.addToWaiting('user')).toBe(true);
        expect(await db.isWaiting('user')).toBe(true);
    });

    test('adding to waiting twice', async () => {
        expect(await db.addToWaiting('user')).toBe(true);
        expect(await db.addToWaiting('user')).toBe(false);
    });

    test('delete from waiting', async () => {
        expect(await db.deleteFromWaiting('user')).toBe(true);
        expect(await db.addToWaiting('user')).toBe(true);
        expect(await db.isWaiting('user')).toBe(true);
        expect(await db.deleteFromWaiting('user')).toBe(true);
        expect(await db.isWaiting('user')).toBe(false);
    });
});

describe('session system', () => {
    test('setting session', async () => {
        expect(await db.addUser('user', 'hashed_password')).toBe(true);
        const token = await db.addSession('user');
        expect(token).toBeTruthy();
        expect(token).toBeTypeOf('string');
        expect(await db.getLoginFromToken(token)).toBe('user');
    });

    test('deleting session', async () => {
        expect(await db.addUser('user', 'hashed_password')).toBe(true);
        const token = await db.addSession('user');
        expect(await db.getLoginFromToken(token)).toBe('user');
        expect(await db.deleteSession(token)).toBe(true);
        expect(await db.getLoginFromToken(token)).toBe(null);
    });

    test('token exists', async () => {
        expect(await db.addUser('user', 'hashed_password')).toBe(true);
        const token1 = await db.addSession('user');
        expect(await db.addUser('user1', 'hashed_password')).toBe(true);
        const token2 = await db.addSession('user1');
        expect(await db.tokenExists(token2)).toBe(true);
    });
});

describe('statistics', () => {
    test('get matches', async () => {
        let matches = [
            { players: ['albert', 'zbychu'], winner: 'albert' },
            { players: ['user1', 'albert'], winner: 'user1' },
            { players: ['user3', 'user4'], winner: 'user3' },
        ];
        let canceled = [false, true, false];

        const created = new Set();
        for (let i = 0; i < matches.length; i++) {
            for (let j = 0; j < matches[i].players.length; j++) {
                if (created.has(matches[i].players[j])) continue;
                created.add(matches[i].players[j]);
                await helperSql`INSERT INTO users (login, password) VALUES (${matches[i].players[j]}, '123')`;
            }
        }
        for (let i = 0; i < matches.length; i++) {
            let q = await helperSql`
                INSERT INTO matches (host, port, results, canceled)
                VALUES ('hoscik', 1233, ${helperSql.json(matches[i])}, ${canceled[i]})
                RETURNING id
            `;
            const match_id = q[0].id;
            for (let j = 0; j < matches[i].players.length; j++) {
                await helperSql`INSERT INTO user_matches (user_id, match_id) VALUES (${matches[i].players[j]}, ${match_id})`;
            }
        }
        const result = await db.getMatchResults('albert');
        expect(result).toEqual([
            {
                details: { players: ['albert', 'zbychu'], winner: 'albert' },
                canceled: false,
            },
            {
                details: { players: ['user1', 'albert'], winner: 'user1' },
                canceled: true,
            },
        ]);
    });
});
