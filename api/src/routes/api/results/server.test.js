import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import * as validate from '$lib/validate.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test/utils.js';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
        getMatchResults: vi.fn(),
    };
});

vi.spyOn(validate, 'validateSession');

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns error when session cookie is missing', async () => {
        const cookies = { get: () => undefined };
        const response = await api.GET({ cookies });

        await expectError(response, ERRORS.noSessionToken);

        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
        expect(db.getMatchResults).not.toHaveBeenCalled();
    });

    it('returns error when session does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.GET({
            cookies: { get: () => EXAMPLE_SESSION_TOKEN },
        });

        await expectError(response, ERRORS.invalidSessionToken);

        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
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
            cookies: { get: () => EXAMPLE_SESSION_TOKEN },
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

        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
        expect(db.getMatchResults).toHaveBeenCalledWith('user1');
    });
});
