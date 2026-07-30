import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';

vi.mock('$lib/db.js', () => {
    return {
        deleteSession: vi.fn(),
    };
});

describe('POST', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns 200 and removes session from db and user cookies', async () => {
        const cookies = {
            get: vi.fn().mockReturnValue('session-token-123'),
            delete: vi.fn(),
        };
        const response = await api.POST({ cookies });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({});
        expect(cookies.get).toHaveBeenCalledWith('session');
        expect(db.deleteSession).toHaveBeenCalledWith('session-token-123');
        expect(cookies.delete).toHaveBeenCalledWith('session', { path: '/' });
    });

    it('returns error when no session cookie', async () => {
        const cookies = {
            get: vi.fn().mockReturnValue(undefined),
            delete: vi.fn(),
        };
        const response = await api.POST({ cookies });

        await expectError(response, ERRORS.noSessionToken);
        expect(db.deleteSession).not.toHaveBeenCalled();
        expect(cookies.delete).not.toHaveBeenCalled();
    });
});
