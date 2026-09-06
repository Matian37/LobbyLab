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
    });

    it('deletes the stale cookie when the session does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue(null);
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN, delete: vi.fn() };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ login: null });
        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
        expect(cookies.delete).toHaveBeenCalledWith('session', { path: '/' });
    });

    it('returns the login when the session exists', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN, delete: vi.fn() };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ login: 'user1' });
        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
        expect(cookies.delete).not.toHaveBeenCalled();
    });
});
