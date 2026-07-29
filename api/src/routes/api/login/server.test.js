import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from '$lib/constants.js';

const EXAMPLE_LOGIN = 'a'.repeat(LOGIN_MIN_LENGTH);
const EXAMPLE_PASSWORD = 'a'.repeat(PASSWORD_MIN_LENGTH);

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
            request: mockRequest({ login: 123, password: EXAMPLE_PASSWORD }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.verifyPassword).not.toHaveBeenCalled();
    });

    it('returns failure when password is not a string', async () => {
        const response = await api.POST({
            request: mockRequest({ login: EXAMPLE_LOGIN, password: 123 }),
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
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: EXAMPLE_PASSWORD,
            }),
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

    it('returns failure when verifyPassword return false', async () => {
        db.verifyPassword.mockResolvedValue(false);

        const response = await api.POST({
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: EXAMPLE_PASSWORD,
            }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Invalid login or password',
        });
    });
});
