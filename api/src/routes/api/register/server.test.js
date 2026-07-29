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
            request: mockRequest({ login: 123, password: EXAMPLE_PASSWORD }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is not a string', async () => {
        const response = await api.POST({
            request: mockRequest({ login: EXAMPLE_LOGIN, password: 123 }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login and password must be strings',
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is too short', async () => {
        const response = await api.POST({
            request: mockRequest({ login: EXAMPLE_LOGIN, password: '' }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: `Password must be at least ${PASSWORD_MIN_LENGTH} and at most ${PASSWORD_MAX_LENGTH} characters`,
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when password is too long', async () => {
        const response = await api.POST({
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: 'x'.repeat(PASSWORD_MAX_LENGTH + 1),
            }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: `Password must be at least ${PASSWORD_MIN_LENGTH} and at most ${PASSWORD_MAX_LENGTH} characters`,
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when login is too short', async () => {
        const response = await api.POST({
            request: mockRequest({ login: '', password: EXAMPLE_PASSWORD }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: `Login must be at least ${LOGIN_MIN_LENGTH} and at most ${LOGIN_MAX_LENGTH} characters`,
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('returns failure when login is too long', async () => {
        const response = await api.POST({
            request: mockRequest({
                login: 'x'.repeat(LOGIN_MAX_LENGTH + 1),
                password: EXAMPLE_PASSWORD,
            }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: `Login must be at least ${LOGIN_MIN_LENGTH} and at most ${LOGIN_MAX_LENGTH} characters`,
        });
        expect(db.addUser).not.toHaveBeenCalled();
    });

    it('registers user and sets session cookie', async () => {
        db.addUser.mockResolvedValue(true);
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
        expect(db.addUser).toHaveBeenCalledWith(
            EXAMPLE_LOGIN,
            EXAMPLE_PASSWORD
        );
        expect(db.addSession).toHaveBeenCalledWith(EXAMPLE_LOGIN);
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
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: EXAMPLE_PASSWORD,
            }),
        });

        expect(await response.json()).toEqual({
            success: false,
            msg: 'Login is already taken',
        });
        expect(db.addSession).not.toHaveBeenCalled();
    });
});
