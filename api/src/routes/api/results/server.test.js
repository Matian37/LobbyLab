import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';

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

    it('returns error when session cookie is missing', async () => {
        const response = await api.GET({ cookies: mockCookies(undefined) });

        await expectError(response, ERRORS.noSessionToken);
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
    });

    it('returns error when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.GET({
            cookies: mockCookies('invalid-token'),
        });

        await expectError(response, ERRORS.invalidSessionToken);
        expect(db.getLoginFromToken).toHaveBeenCalledWith('invalid-token');
        expect(db.getMatchResults).not.toHaveBeenCalled();
    });

    it('returns 200 with matches when user is logged in', async () => {
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

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({
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
});
