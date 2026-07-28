import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        verifyPassword: vi.fn(),
        addSession: vi.fn(),
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

    it('returns failure when login is not a string', async () => {
        const response = await api.POST({
            request: mockRequest({ login: 123, password: 'correct' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.verifyPassword).not.toHaveBeenCalled();
    });

    it('returns failure when password is not a string', async () => {
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 123 }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.verifyPassword).not.toHaveBeenCalled();
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
