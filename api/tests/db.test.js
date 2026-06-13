import { expect, test, beforeEach, describe, beforeAll } from 'vitest';
import * as db from '$lib/db.js';
import bcrypt from 'bcryptjs';
beforeEach(async () => {
    await db.truncateEverything();
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

    test('adding to waiting once', async () => {
        expect(await db.addToWaiting('user')).toBe(true);
        expect((await db.findWaitingByLogin('user')).length).toBe(1);
    });
        
    
    test('adding to waiting twice', async () => {
        expect(await db.addToWaiting('user')).toBe(true);
        expect((await db.addToWaiting('user'))).toBe(false);
    });

    test('delete from waiting', async () => {
        expect(await db.deleteFromWaiting('user')).toBe(true);
        expect(await db.addToWaiting('user')).toBe(true);
        expect((await db.findWaitingByLogin('user')).length).toBe(1);
        expect(await db.deleteFromWaiting('user')).toBe(true);
        expect((await db.findWaitingByLogin('user')).length).toBe(0);
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