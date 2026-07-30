import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from '$lib/constants.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test-utils.js';

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

    it.each([
        {
            name: 'login is not a string',
            login: 123,
            password: EXAMPLE_PASSWORD,
            error: ERRORS.invalidCredentialTypes,
        },
        {
            name: 'password is not a string',
            login: EXAMPLE_LOGIN,
            password: 123,
            error: ERRORS.invalidCredentialTypes,
        },
        {
            name: 'login is too short',
            login: '',
            password: EXAMPLE_PASSWORD,
            error: ERRORS.invalidCredentials,
        },
        {
            name: 'login is too long',
            login: 'x'.repeat(LOGIN_MAX_LENGTH + 1),
            password: EXAMPLE_PASSWORD,
            error: ERRORS.invalidCredentials,
        },
        {
            name: 'password is too short',
            login: EXAMPLE_LOGIN,
            password: '',
            error: ERRORS.invalidCredentials,
        },
        {
            name: 'password is too long',
            login: EXAMPLE_LOGIN,
            password: 'x'.repeat(PASSWORD_MAX_LENGTH + 1),
            error: ERRORS.invalidCredentials,
        },
    ])('returns error when $name', async ({ login, password, error }) => {
        const response = await api.POST({
            request: mockRequest({ login, password }),
        });

        await expectError(response, error);
        expect(db.verifyPassword).not.toHaveBeenCalled();
        expect(db.addSession).not.toHaveBeenCalled();
    });

    it('returns 200 and sets session cookie when credentials are valid', async () => {
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

        expect(response.status).toBe(200);
        expect(await response.json()).toEqual({});
        expect(db.verifyPassword).toHaveBeenCalledWith(
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

    it('returns error when credentials are invalid', async () => {
        db.verifyPassword.mockResolvedValue(false);

        const response = await api.POST({
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: EXAMPLE_PASSWORD,
            }),
        });

        await expectError(response, ERRORS.invalidCredentials);
        expect(db.verifyPassword).toHaveBeenCalledWith(
            EXAMPLE_LOGIN,
            EXAMPLE_PASSWORD
        );
        expect(db.addSession).not.toHaveBeenCalled();
    });

    it('returns error when user gone during request', async () => {
        db.verifyPassword.mockResolvedValue(true);
        db.addSession.mockResolvedValue(null);

        const cookies = { set: vi.fn() };
        const response = await api.POST({
            request: mockRequest({
                login: EXAMPLE_LOGIN,
                password: EXAMPLE_PASSWORD,
            }),
            cookies,
        });

        await expectError(response, ERRORS.invalidCredentials);

        expect(db.verifyPassword).toHaveBeenCalledWith(
            EXAMPLE_LOGIN,
            EXAMPLE_PASSWORD
        );
        expect(db.addSession).toHaveBeenCalled();
        expect(cookies.set).not.toHaveBeenCalled();
    });
});
