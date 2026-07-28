import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => {
    return {
        addUser: vi.fn(),
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
            request: mockRequest({ login: 123, password: 'secret' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is not a string', async () => {
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 123 }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is too short', async () => {
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: '1234' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Password must be between 5 and 64 characters',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is too long', async () => {
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 'x'.repeat(65) }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Password must be between 5 and 64 characters',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('registers user and sets session cookie', async () => {
        db.addUser.mockResolvedValue(true);
        db.addSession.mockResolvedValue('session-token-123');

        const cookies = { set: vi.fn() };
        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 'secret' }),
            cookies,
        });

        expect(await response.json()).toEqual({ success: true, msg: null });
        expect(db.addUser).toHaveBeenCalledWith('alice', 'secret');
        expect(db.addSession).toHaveBeenCalledWith('alice');
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

    it('returns failure when login is already taken', async () => {
        db.addUser.mockResolvedValue(false);

        const response = await api.POST({
            request: mockRequest({ login: 'alice', password: 'secret' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login is already taken',
        });
        expect(db.addSession).not.toHaveBeenCalled();
    });
});
