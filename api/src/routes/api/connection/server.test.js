import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as validate from '$lib/validate.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);

vi.spyOn(validate, 'validateSession');

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns error when session cookie is missing', async () => {
        const cookies = { get: () => undefined };
        const response = api.GET({ cookies });

        await expectError(response, ERRORS.noSessionToken);
        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
    });

    it('returns error when session token is invalid', async () => {
        const cookies = { get: () => 'not-a-valid-token' };
        const response = api.GET({ cookies });

        await expectError(response, ERRORS.invalidSessionToken);
        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
    });

    it('returns 426 when the session is valid', async () => {
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };
        const response = api.GET({ cookies });

        expect(response.status).toBe(426);
        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
    });
});
