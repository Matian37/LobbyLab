import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import * as validate from '$lib/validate.js';
import { PASSWORD_MIN_LENGTH, LOGIN_MIN_LENGTH } from '$lib/constants.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test/utils.js';

const EXAMPLE_LOGIN = 'a'.repeat(LOGIN_MIN_LENGTH);
const EXAMPLE_PASSWORD = 'a'.repeat(PASSWORD_MIN_LENGTH);

vi.mock('$lib/db.js', () => {
    return {
        verifyPassword: vi.fn(),
        addSession: vi.fn(),
    };
});

vi.spyOn(validate, 'validateCredentialsSchema');

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

    it('returns error when request body is invalid', async () => {
        const request = mockRequest({});
        const response = await api.POST({ request });

        await expectError(response, ERRORS.missingLogin);

        expect(validate.validateCredentialsSchema).toHaveBeenCalledWith(
            request
        );
        expect(db.verifyPassword).not.toHaveBeenCalled();
        expect(db.addSession).not.toHaveBeenCalled();
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

        expect(validate.validateCredentialsSchema).toHaveBeenCalled();
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

        expect(validate.validateCredentialsSchema).toHaveBeenCalled();
        expect(db.verifyPassword).toHaveBeenCalledWith(
            EXAMPLE_LOGIN,
            EXAMPLE_PASSWORD
        );
        expect(db.addSession).toHaveBeenCalled();
        expect(cookies.set).not.toHaveBeenCalled();
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

        expect(validate.validateCredentialsSchema).toHaveBeenCalled();
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
});
