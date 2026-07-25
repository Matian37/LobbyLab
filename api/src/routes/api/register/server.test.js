import { vi, test, expect, describe } from 'vitest';
import * as registerAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        addUser: vi.fn().mockResolvedValue(true),
        tokenExists: vi.fn().mockResolvedValue([]),
        addSession: vi.fn().mockResolvedValue(true),
    };
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
        expect(result).toEqual({ sukces: false, msg: "Podany login jest zajęty" });
    });

    test('registering correctly', async () => {
        const request = new Request('http://cos', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ login: 'user', password: 'haslo' })
        });

        const response = await registerAPI.POST({
            request,
            cookies: {
                set: vi.fn()
            }
        });
        const result = await response.json();
        expect(result.sukces).toBe(true);
        expect(result.msg).toBe(null);
    });
});
