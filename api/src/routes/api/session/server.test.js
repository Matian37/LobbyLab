import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import * as validate from '$lib/validate.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);

vi.mock('$lib/db.js', () => {
    return {
        sessionExist: vi.fn(),
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
        expect(db.sessionExist).not.toHaveBeenCalled();
    });

    it('returns 200 with exists: true when session exists', async () => {
        db.sessionExist.mockResolvedValue(true);

        const response = await api.GET({
            cookies: { get: () => EXAMPLE_SESSION_TOKEN },
        });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ exists: true });
        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.sessionExist).toHaveBeenCalledWith(EXAMPLE_SESSION_TOKEN);
    });

    it('returns 200 with exists: false when session does not exist', async () => {
        db.sessionExist.mockResolvedValue(false);

        const response = await api.GET({
            cookies: { get: () => EXAMPLE_SESSION_TOKEN },
        });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ exists: false });
        expect(validate.validateSession).toHaveBeenCalled();
        expect(db.sessionExist).toHaveBeenCalledWith(EXAMPLE_SESSION_TOKEN);
    });
});
