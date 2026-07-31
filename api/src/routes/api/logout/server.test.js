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
        deleteSession: vi.fn(),
    };
});

vi.spyOn(validate, 'validateSession');

describe('POST', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns error when session cookie is missing', async () => {
        const cookies = { get: () => undefined, delete: vi.fn() };
        const response = await api.POST({ cookies });

        await expectError(response, ERRORS.noSessionToken);

        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
        expect(db.deleteSession).not.toHaveBeenCalled();
        expect(cookies.delete).not.toHaveBeenCalled();
    });

    it('returns 200 and removes session from db and user cookies', async () => {
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN, delete: vi.fn() };
        const response = await api.POST({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({});

        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
        expect(db.deleteSession).toHaveBeenCalledWith(EXAMPLE_SESSION_TOKEN);
        expect(cookies.delete).toHaveBeenCalledWith('session', { path: '/' });
    });
});
