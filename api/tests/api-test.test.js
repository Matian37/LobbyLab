import { vi, test, expect, describe, beforeEach } from 'vitest';
import * as loginAPI from '$routes/api/login/+server.js';
import * as registerAPI from '$routes/api/register/+server.js';
import * as waitingAPI from '$routes/api/waiting/+server.js';
import * as resultsAPI from '$routes/api/results/+server.js'
import * as db from '$lib/db.js';
import bcrypt from 'bcryptjs';


vi.mock('$lib/db.js', () => {
    const bcrypt = require('bcryptjs')
    return {
        findUserByLogin: vi.fn().mockResolvedValue([{ login: 'user', password: bcrypt.hashSync("123", 10) }]),
        tokenExists: vi.fn().mockResolvedValue([]),
        setSession: vi.fn().mockResolvedValue(true),
        addUser: vi.fn().mockResolvedValue(true),
        getLoginFromToken: vi.fn().mockResolvedValue([{login: "user"}]),
        addToWaiting: vi.fn().mockResolvedValue(true),
        deleteFromWaiting: vi.fn().mockResolvedValue(true),
        getMatchResults: vi.fn().mockResolvedValue([
            {players: ['albert', 'zbychu'], winner: 'albert'},
            {players: ['user1', 'albert'], winner: 'user1'},
        ]),
    }
})

describe('logging in', () => {
    test('logging in incorrectly (invalid login)', async () => {
        db.findUserByLogin.mockResolvedValueOnce([]);

        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' })
        });

        const response = await loginAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({sukces: false, msg: "Podany login nie istnieje"});
    });

    test('logging in incorrectly (invalid password)', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: 'zle haslo' })
        });

        const response = await loginAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({ sukces: false, msg: "Podane hasło jest błędne" });
    });

    test('logging in correctly', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' })
        });

        const response = await loginAPI.POST({ request,
            cookies: {
                set: vi.fn()
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(true);
    });
});


describe('registering', () => {
    test('registering incorrectly (invalid login)', async () => {
        db.addUser.mockResolvedValueOnce(false);

        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' })
        });

        const response = await registerAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({sukces: false, msg: "Podany login jest zajęty"});
    });

    test('registering correctly', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: 'haslo' })
        });

        const response = await registerAPI.POST({ request,
            cookies: {
                set: vi.fn()
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(true);
        expect(result.msg).toBe(null);
    });
});


describe('waiting', () => {
    test('adding to waiting user with wrong token', async () => {
        db.getLoginFromToken.mockResolvedValueOnce([]);

        const response = await waitingAPI.POST({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(false);
    });

    test('adding to waiting already waiting user', async () => {
        db.addToWaiting.mockResolvedValueOnce(false);

        const response = await waitingAPI.POST({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(false);
    });

    test('adding to waiting correctly', async () => {
        const response = await waitingAPI.POST({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(true);
    });

    test('deleting from waiting user with no corresponding login to token', async () => {
        db.getLoginFromToken.mockResolvedValueOnce([]);

        const response = await waitingAPI.DELETE({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(false);
    });

    test('deleting from waiting', async () => {

        const response = await waitingAPI.DELETE({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
         });
        const result = await response.json();
        expect(result.sukces).toBe(true);
    });
});

describe('statistics', () => {
    test('getting matches', async () => {
        const response = await resultsAPI.GET( {
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                }
            }
        } );
        const result = await response.json();
        expect(result).toEqual({sukces: true, matches: [
            {players: ['albert', 'zbychu'], winner: 'albert'},
            {players: ['user1', 'albert'], winner: 'user1'},
        ]});
    });
});
