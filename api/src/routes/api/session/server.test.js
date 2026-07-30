import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';

vi.mock('$lib/db.js', () => {
    return {
        sessionExist: vi.fn(),
    };
});

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns 200 with exists: true when session token is valid', async () => {
        db.sessionExist.mockResolvedValue(true);

        const cookies = { get: vi.fn().mockReturnValue('session-token-123') };
        const response = await api.GET({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ exists: true });
    });

    it('returns 200 with exists: false when session token is invalid', async () => {
        db.sessionExist.mockResolvedValue(false);

        const cookies = { get: vi.fn().mockReturnValue('invalid-token') };
        const response = await api.GET({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ exists: false });
    });

    it('returns error when no session cookie', async () => {
        const cookies = { get: vi.fn().mockReturnValue(undefined) };
        const response = await api.GET({ cookies });

        await expectError(response, ERRORS.noSessionToken);
        expect(db.sessionExist).not.toHaveBeenCalled();
    });
});
