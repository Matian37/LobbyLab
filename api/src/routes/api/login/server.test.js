import { vi, test, expect, describe } from 'vitest';
import * as loginAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        verifyPassword: vi
            .fn()
            .mockImplementation(async (login, password) => password === '123'),
        tokenExists: vi.fn().mockResolvedValue(true),
        addSession: vi.fn().mockResolvedValue(true),
    };
});

describe('logging in', () => {
    test('logging in incorrectly (invalid login)', async () => {
        db.verifyPassword.mockResolvedValueOnce(false);

        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' }),
        });

        const response = await loginAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({
            success: false,
            msg: 'Invalid login or password',
        });
    });

    test('logging in incorrectly (invalid password)', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: 'wrong password' }),
        });

        const response = await loginAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({
            success: false,
            msg: 'Invalid login or password',
        });
    });

    test('logging in correctly', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' }),
        });

        const response = await loginAPI.POST({
            request,
            cookies: {
                set: vi.fn(),
            },
        });
        const result = await response.json();
        expect(result.success).toBe(true);
    });
});
