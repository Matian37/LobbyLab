import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
        getMatchResults: vi.fn(),
    };
});

function mockCookies(token) {
    return { get: vi.fn().mockReturnValue(token) };
}

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns matches for a logged-in user', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.getMatchResults.mockResolvedValue([
            {
                details: { players: ['user1', 'user2'], winner: 'user1' },
                canceled: false,
            },
            {
                details: { players: ['user1', 'user3'], winner: 'user1' },
                canceled: true,
            },
        ]);

        const response = await api.GET({
            cookies: mockCookies('session-token-123'),
        });

        expect(await response.json()).toEqual({
            success: true,
            matches: [
                {
                    details: { players: ['user1', 'user2'], winner: 'user1' },
                    canceled: false,
                },
                {
                    details: { players: ['user1', 'user3'], winner: 'user1' },
                    canceled: true,
                },
            ],
        });
        expect(db.getLoginFromToken).toHaveBeenCalledWith('session-token-123');
        expect(db.getMatchResults).toHaveBeenCalledWith('user1');
    });

    it('returns failure when session cookie is missing', async () => {
        const response = await api.GET({ cookies: mockCookies(undefined) });

        expect(await response.json()).toEqual({
            success: false,
            matches: null,
        });
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
    });

    it('returns failure when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.GET({
            cookies: mockCookies('invalid-token'),
        });

        expect(await response.json()).toEqual({
            success: false,
            matches: null,
        });
        expect(db.getMatchResults).not.toHaveBeenCalled();
    });
});
