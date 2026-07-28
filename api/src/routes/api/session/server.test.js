import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        sessionExist: vi.fn(),
    };
});

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns true when session token is valid', async () => {
        db.sessionExist.mockResolvedValue(true);

        const cookies = { get: vi.fn().mockReturnValue('session-token-123') };
        const response = await api.GET({ cookies });

        expect(await response.json()).toEqual({ success: true });
    });

    it('returns false when session token is invalid', async () => {
        db.sessionExist.mockResolvedValue(false);

        const cookies = { get: vi.fn().mockReturnValue('invalid-token') };
        const response = await api.GET({ cookies });

        expect(await response.json()).toEqual({ success: false });
    });

    it('returns false when no session cookie', async () => {
        const cookies = { get: vi.fn().mockReturnValue(undefined) };
        const response = await api.GET({ cookies });

        expect(await response.json()).toEqual({ success: false });
        expect(db.sessionExist).not.toHaveBeenCalled();
    });
});
