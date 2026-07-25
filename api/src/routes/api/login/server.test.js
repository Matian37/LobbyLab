import { vi, test, expect, describe } from 'vitest';
import * as loginAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    const bcrypt = require('bcryptjs');
    return {
        findUserByLogin: vi.fn().mockResolvedValue([{ login: 'user', password: bcrypt.hashSync("123", 10) }]),
        tokenExists: vi.fn().mockResolvedValue([]),
        setSession: vi.fn().mockResolvedValue(true),
    };
});

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
        expect(result).toEqual({ sukces: false, msg: "Podany login nie istnieje" });
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

        const response = await loginAPI.POST({
            request,
            cookies: {
                set: vi.fn()
            }
        });
        const result = await response.json();
        expect(result.sukces).toBe(true);
    });
});
