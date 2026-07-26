import { vi, test, expect, describe } from 'vitest';
import * as registerAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        addUser: vi.fn().mockResolvedValue(true),
        tokenExists: vi.fn().mockResolvedValue(true),
        addSession: vi.fn().mockResolvedValue(true),
    };
});

describe('registering', () => {
    test('registering incorrectly (invalid login)', async () => {
        db.addUser.mockResolvedValueOnce(false);

        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: '123' }),
        });

        const response = await registerAPI.POST({ request });
        const result = await response.json();
        expect(result).toEqual({
            success: false,
            msg: 'Login is already taken',
        });
    });

    test('registering correctly', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: 'secret' }),
        });

        const response = await registerAPI.POST({
            request,
            cookies: {
                set: vi.fn(),
            },
        });
        const result = await response.json();
        expect(result.success).toBe(true);
        expect(result.msg).toBe(null);
    });
});
