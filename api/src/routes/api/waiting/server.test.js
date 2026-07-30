import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
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

    it('returns 200 with waiting: true when user is waiting', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.isWaiting.mockResolvedValue(true);

        const response = await api.GET({ cookies: mockCookies('token-123') });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ waiting: true });
        expect(db.getLoginFromToken).toHaveBeenCalledWith('token-123');
        expect(db.isWaiting).toHaveBeenCalledWith('user1');
    });

    it('returns 200 with waiting: false when user is not waiting', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.isWaiting.mockResolvedValue(false);

        const response = await api.GET({ cookies: mockCookies('token-123') });

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({ waiting: false });
        expect(db.getLoginFromToken).toHaveBeenCalledWith('token-123');
        expect(db.isWaiting).toHaveBeenCalledWith('user1');
    });

    it('returns error when session cookie is missing', async () => {
        const response = await api.GET({ cookies: mockCookies(undefined) });

        await expectError(response, ERRORS.noSessionToken);
        expect(db.isWaiting).not.toHaveBeenCalled();
    });

    it('returns error when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.GET({ cookies: mockCookies('invalid') });

        await expectError(response, ERRORS.invalidSessionToken);
        expect(db.isWaiting).not.toHaveBeenCalled();
    });
});
