import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
        addToWaiting: vi.fn(),
        deleteFromWaiting: vi.fn(),
        isWaiting: vi.fn(),
    };
});

function mockCookies(token) {
    return { get: vi.fn().mockReturnValue(token) };
}

describe('POST', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('adds user to waiting queue when logged in', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.addToWaiting.mockResolvedValue(true);

        const response = await api.POST({ cookies: mockCookies('token-123') });

        expect(await response.json()).toEqual({ success: true });
        expect(db.addToWaiting).toHaveBeenCalledWith('user1');
    });

    it('returns failure when session cookie is missing', async () => {
        const response = await api.POST({ cookies: mockCookies(undefined) });

        expect(await response.json()).toEqual({ success: false });
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
    });

    it('returns failure when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.POST({ cookies: mockCookies('invalid') });

        expect(await response.json()).toEqual({ success: false });
        expect(db.addToWaiting).not.toHaveBeenCalled();
    });

    it('returns failure when user is already waiting', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.addToWaiting.mockResolvedValue(false);

        const response = await api.POST({ cookies: mockCookies('token-123') });

        expect(await response.json()).toEqual({ success: false });
    });
});

describe('DELETE', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('removes user from waiting queue when logged in', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.deleteFromWaiting.mockResolvedValue(true);

        const response = await api.DELETE({
            cookies: mockCookies('token-123'),
        });

        expect(await response.json()).toEqual({ success: true });
        expect(db.deleteFromWaiting).toHaveBeenCalledWith('user1');
    });

    it('returns failure when session cookie is missing', async () => {
        const response = await api.DELETE({ cookies: mockCookies(undefined) });

        expect(await response.json()).toEqual({ success: false });
        expect(db.deleteFromWaiting).not.toHaveBeenCalled();
    });

    it('returns failure when session token is invalid', async () => {
        db.getLoginFromToken.mockResolvedValue(null);

        const response = await api.DELETE({ cookies: mockCookies('invalid') });

        expect(await response.json()).toEqual({ success: false });
        expect(db.deleteFromWaiting).not.toHaveBeenCalled();
    });

    it('returns failure when user is not in the queue', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.deleteFromWaiting.mockResolvedValue(false);

        const response = await api.DELETE({
            cookies: mockCookies('token-123'),
        });

        expect(await response.json()).toEqual({ success: false });
    });
});

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
