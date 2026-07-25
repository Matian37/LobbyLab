import { vi, test, expect, describe } from 'vitest';
import * as waitingAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn().mockResolvedValue([{ login: 'user' }]),
        addToWaiting: vi.fn().mockResolvedValue(true),
        deleteFromWaiting: vi.fn().mockResolvedValue(true),
    };
});

describe('waiting', () => {
    test('adding to waiting user with wrong token', async () => {
        db.getLoginFromToken.mockResolvedValueOnce([]);

        const response = await waitingAPI.POST({
            cookies: {
                get: (name) => {
                    if (name === 'token') return '1234567';
                    return undefined;
                },
            },
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
                },
            },
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
                },
            },
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
                },
            },
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
                },
            },
        });
        const result = await response.json();
        expect(result.sukces).toBe(true);
    });
});
