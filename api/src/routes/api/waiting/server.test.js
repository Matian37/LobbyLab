import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
        isWaiting: vi.fn(),
        deleteFromWaiting: vi.fn(),
        isWaiting: vi.fn(),
    };
});

function mockCookies(token) {
    return { get: vi.fn().mockReturnValue(token) };
}

describe('GET', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns true when user is waiting', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.isWaiting.mockResolvedValue(true);

        const response = await api.GET({ cookies: mockCookies('token-123') });

        expect(await response.json()).toEqual({ success: true });
    });

    it('returns false when user is not waiting', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.isWaiting.mockResolvedValue(false);

        const response = await api.GET({ cookies: mockCookies('token-123') });

        expect(await response.json()).toEqual({ success: false });
    });

    it('returns failure when session cookie is missing', async () => {
        const response = await api.GET({ cookies: mockCookies(undefined) });

        expect(await response.json()).toEqual({ success: false });
        expect(db.isWaiting).not.toHaveBeenCalled();
    });

    it('returns failure when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.GET({ cookies: mockCookies('invalid') });

        expect(await response.json()).toEqual({ success: false });
        expect(db.isWaiting).not.toHaveBeenCalled();
    });
});
