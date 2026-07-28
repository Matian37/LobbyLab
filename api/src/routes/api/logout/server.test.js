import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        deleteSession: vi.fn(),
    };
});

describe('POST', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('deletes session and clears cookie when token exists', async () => {
        db.deleteSession.mockResolvedValue(true);

        const cookies = {
            get: vi.fn().mockReturnValue('session-token-123'),
            delete: vi.fn(),
        };
        const response = await api.POST({ cookies });

        expect(await response.json()).toEqual({ success: true });
        expect(db.deleteSession).toHaveBeenCalledWith('session-token-123');
        expect(cookies.delete).toHaveBeenCalledWith('session', { path: '/' });
    });

    it('returns failure when no session cookie', async () => {
        const cookies = { get: vi.fn().mockReturnValue(undefined) };
        const response = await api.POST({ cookies });

        expect(await response.json()).toEqual({ success: false });
        expect(db.deleteSession).not.toHaveBeenCalled();
    });
});
