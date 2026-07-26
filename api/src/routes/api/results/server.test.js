import { vi, test, expect, describe } from 'vitest';
import * as resultsAPI from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn().mockResolvedValue([{ login: 'user' }]),
        getMatchResults: vi.fn().mockResolvedValue([
            {
                details: { players: ['albert', 'zbychu'], winner: 'albert' },
                canceled: false,
            },
            {
                details: { players: ['user1', 'albert'], winner: 'user1' },
                canceled: true,
            },
        ]),
    };
});

describe('statistics', () => {
    test('getting matches', async () => {
        const response = await resultsAPI.GET({
            cookies: {
                get: (name) => {
                    if (name === 'session') return '1234567';
                    return undefined;
                },
            },
        });
        const result = await response.json();
        expect(result).toEqual({
            success: true,
            matches: [
                {
                    details: {
                        players: ['albert', 'zbychu'],
                        winner: 'albert',
                    },
                    canceled: false,
                },
                {
                    details: { players: ['user1', 'albert'], winner: 'user1' },
                    canceled: true,
                },
            ],
        });
    });
});
