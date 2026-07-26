import { expect, test, beforeEach, describe } from 'vitest';
import * as db from '$lib/db.js';
import { sql } from '$lib/db.js';
import postgres from 'postgres';

beforeEach(async () => {
    const sql = postgres(process.env.DATABASE_URL);
    await sql.unsafe('DROP SCHEMA public CASCADE; CREATE SCHEMA public;');
    await sql.unsafe(process.env.DATABASE_INIT_SQL);
    await sql.end();
});

describe('user adding and getting', () => {
    test('normal user adding', async () => {
        expect(await db.addUser('user', 'hashedPassword')).toBe(true);
        expect((await db.findUserByLogin('user')).length).toBe(1);
    });

    test('user adding twice', async () => {
        expect(await db.addUser('user', 'hashedPassword')).toBe(true);
        expect(await db.addUser('user', 'elo')).toBe(false);
    });

});

describe('waiting list', () => {
    test('adding to waiting', async () => {
        expect(await db.addUser('user', 'hashedPassword')).toBe(true);
        expect((await db.findWaitingByLogin('user')).length).toBe(0);
        expect(await db.setUserStatus('user')).toBe(true);
        expect((await db.findWaitingByLogin('user')).length).toBe(1);
    });
});

describe('session system', () => {

    test('setting session', async () => {
        expect((await db.addUser('user', 'hashed_password'))).toBe(true);
        expect((await db.getLoginFromToken('1234567')).length).toBe(0);
        expect(await db.setSession('1234567', 'user')).toBe(true);
        expect((await db.getLoginFromToken('1234567'))[0].login).toBe('user');
    });


    test('deleting session', async () => {
        expect((await db.addUser('user', 'hashed_password'))).toBe(true);
        expect(await db.setSession('1234567', 'user')).toBe(true);
        expect((await db.getLoginFromToken("1234567")).length).toBe(1);
        expect(await db.deleteSession('1234567')).toBe(true);
        expect((await db.getLoginFromToken("1234567")).length).toBe(0);
    });

    test('token exists', async () => {
        expect((await db.addUser('user', 'hashed_password'))).toBe(true);
        expect(await db.setSession('1234567', 'user')).toBe(true);
        expect((await db.addUser('user1', 'hashed_password'))).toBe(true);
        expect(await db.setSession('12345678', 'user1')).toBe(true);
        expect((await db.tokenExists('12345678')).length).toBe(1);
    });
});

describe('statistics', () => {
    test('get matches', async () => {
        let matches = [
            {players: ['albert', 'zbychu'], winner: 'albert'},
            {players: ['user1', 'albert'], winner: 'user1'},
            {players: ['user3','user4'], winner: 'user3'},
        ]
        let canceled = [false, true, false];

        const created = new Set();
        for(let i = 0; i < matches.length; i++){
            for(let j = 0; j < matches[i].players.length; j++){
                if(created.has(matches[i].players[j]))
                    continue;
                created.add(matches[i].players[j]);
                await sql`INSERT INTO users (login, password) VALUES (${matches[i].players[j]}, '123')`
            }
        }
        for(let i = 0; i < matches.length; i++){
            let q = await sql`
                INSERT INTO matches (host, port, results, canceled)
                VALUES ('hoscik', 1233, ${sql.json(matches[i])}, ${canceled[i]})
                RETURNING id
            `;
            const match_id = q[0].id;
            for(let j = 0; j < matches[i].players.length; j++){
                await sql`INSERT INTO user_matches (user_id, match_id) VALUES (${matches[i].players[j]}, ${match_id})`
            }
        }
        const result = await db.getMatchResults('albert');
        expect(result).toEqual([
            {
                details: { players: ['albert', 'zbychu'], winner: 'albert' },
                canceled: false
            },
            {
                details: { players: ['user1', 'albert'], winner: 'user1' },
                canceled: true
            },
        ]);
    });
});

describe("getAuthToken", () => {
    test("returns auth token for valid login", async () => {
        await sql`INSERT INTO users (login, password, match_auth_token) VALUES ('user', '123', 'token')`;
        const token = await db.getAuthToken('user');
        expect(token).toBe('token');
    });

    test("returns null for invalid login", async () => {
        const token = await db.getAuthToken('invalid');
        expect(token).toBeNull();
    });

    test("returns null for no token", async () => {
        await sql`INSERT INTO users (login, password) VALUES ('user', '123')`;
        const token = await db.getAuthToken('user');
        expect(token).toBeNull();
    });
});