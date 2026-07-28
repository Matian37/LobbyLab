import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        verifyPassword: vi.fn(),
        addSession: vi.fn(),
        deleteSession: vi.fn(),
        sessionExist: vi.fn(),
    };
});

function mockRequest(body) {
    return new Request('http://abc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
}

describe('POST', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns success and sets session cookie when credentials are valid', async () => {
        db.verifyPassword.mockResolvedValue(true);
        db.addSession.mockResolvedValue('session-token-123');

        const cookies = { set: vi.fn() };
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 'correct' }),
            cookies,
        });

        expect(await response.json()).toEqual({ success: true, msg: null });
        expect(cookies.set).toHaveBeenCalledWith(
            'session',
            'session-token-123',
            expect.objectContaining({
                httpOnly: true,
                secure: true,
                sameSite: 'strict',
            })
        );
    });

    it('returns failure when password does not match', async () => {
        db.verifyPassword.mockResolvedValue(false);

        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 'wrong' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Invalid login or password',
        });
    });

    it('returns failure when login does not exist', async () => {
        db.verifyPassword.mockResolvedValue(false);

        const response = await api.POST({
            request: mockRequest({ login: 'nobody', password: 'anything' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Invalid login or password',
        });
    });
});

describe('DELETE', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('deletes session and clears cookie when token exists', async () => {
        db.deleteSession.mockResolvedValue(true);

        const cookies = {
            get: vi.fn().mockReturnValue('session-token-123'),
            delete: vi.fn(),
        };
        const response = await api.DELETE({ cookies });

        expect(await response.json()).toEqual({ success: true });
        expect(db.deleteSession).toHaveBeenCalledWith('session-token-123');
        expect(cookies.delete).toHaveBeenCalledWith('session', { path: '/' });
    });

    it('returns failure when no session cookie', async () => {
        const cookies = { get: vi.fn().mockReturnValue(undefined) };
        const response = await api.DELETE({ cookies });

        expect(await response.json()).toEqual({ success: false });
        expect(db.deleteSession).not.toHaveBeenCalled();
    });
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
