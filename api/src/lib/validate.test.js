import { it, expect, describe, vi } from 'vitest';
import {
    validateRequest,
    validateCredentialsSchema,
    validateSession,
    isValidToken,
    validateGameLaunchUrl,
} from './validate.js';
import { ERRORS } from './errors.js';
import {
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    SESSION_TOKEN_LENGTH,
} from './constants.js';
import { expectError } from './test/utils.js';

const VALID_LOGIN = 'a'.repeat(LOGIN_MIN_LENGTH);
const VALID_PASSWORD = 'a'.repeat(PASSWORD_MIN_LENGTH);

async function expectErrorResult(result, errorFactory) {
    expect(Object.keys(result)).toEqual(['error']);
    await expectError(result.error, errorFactory);
}

function mockRequest(body) {
    return new Request('http://abc', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
    });
}

describe('validateRequest', () => {
    it.each([
        { name: 'an array', value: [1, 2, 3] },
        { name: 'null', value: null },
        { name: 'a string', value: 'not-json' },
        { name: 'an empty string', value: '' },
        { name: 'a number', value: 123 },
    ])('returns invalidJSON error when body is $name', async ({ value }) => {
        const result = await validateRequest(mockRequest(value));

        await expectErrorResult(result, ERRORS.invalidJSON);
    });

    it('returns body data for a valid object', async () => {
        const body = { login: VALID_LOGIN, password: VALID_PASSWORD };
        const result = await validateRequest(mockRequest(body));

        expect(result).toEqual({ data: body });
    });
});

describe('validateCredentialsSchema', () => {
    it('returns invalidJSON error when request has invalid JSON', async () => {
        const result = await validateCredentialsSchema(mockRequest('not-json'));

        await expectErrorResult(result, ERRORS.invalidJSON);
    });

    it.each([
        {
            name: 'missing login',
            body: { password: VALID_PASSWORD },
            error: ERRORS.missingLogin,
        },
        {
            name: 'missing password',
            body: { login: VALID_LOGIN },
            error: ERRORS.missingPassword,
        },
        {
            name: 'login is not a string',
            body: { login: 123, password: VALID_PASSWORD },
            error: ERRORS.invalidLoginType,
        },
        {
            name: 'password is not a string',
            body: { login: VALID_LOGIN, password: 123 },
            error: ERRORS.invalidPasswordType,
        },
        {
            name: 'login is too short',
            body: {
                login: 'a'.repeat(LOGIN_MIN_LENGTH - 1),
                password: VALID_PASSWORD,
            },
            error: ERRORS.invalidLoginLength,
        },
        {
            name: 'login is too long',
            body: {
                login: 'a'.repeat(LOGIN_MAX_LENGTH + 1),
                password: VALID_PASSWORD,
            },
            error: ERRORS.invalidLoginLength,
        },
        {
            name: 'password is too short',
            body: {
                login: VALID_LOGIN,
                password: 'a'.repeat(PASSWORD_MIN_LENGTH - 1),
            },
            error: ERRORS.invalidPasswordLength,
        },
        {
            name: 'password is too long',
            body: {
                login: VALID_LOGIN,
                password: 'a'.repeat(PASSWORD_MAX_LENGTH + 1),
            },
            error: ERRORS.invalidPasswordLength,
        },
    ])('returns error when $name', async ({ body, error }) => {
        const result = await validateCredentialsSchema(mockRequest(body));

        await expectErrorResult(result, error);
    });

    it('accepts login and password at minimum length', async () => {
        const body = {
            login: VALID_LOGIN,
            password: VALID_PASSWORD,
        };

        expect(await validateCredentialsSchema(mockRequest(body))).toEqual({
            data: body,
        });
    });

    it('accepts login and password at maximum length', async () => {
        const body = {
            login: 'a'.repeat(LOGIN_MAX_LENGTH),
            password: 'a'.repeat(PASSWORD_MAX_LENGTH),
        };

        expect(await validateCredentialsSchema(mockRequest(body))).toEqual({
            data: body,
        });
    });
});

describe('isValidToken', () => {
    it.each([
        {
            name: '64-char token with every possible hex character',
            value: '0123456789abcdef'.repeat(SESSION_TOKEN_LENGTH / 16),
            expected: true,
        },
        {
            name: '64-char uppercase hex',
            value: 'A'.repeat(SESSION_TOKEN_LENGTH),
            expected: false,
        },
        {
            name: 'mixed case hex',
            value: 'aA'.repeat(SESSION_TOKEN_LENGTH / 2),
            expected: false,
        },
        {
            name: 'too short',
            value: 'a'.repeat(SESSION_TOKEN_LENGTH - 1),
            expected: false,
        },
        {
            name: 'too long',
            value: 'a'.repeat(SESSION_TOKEN_LENGTH + 1),
            expected: false,
        },
        {
            name: 'empty string',
            value: '',
            expected: false,
        },
        {
            name: 'non-hex character',
            value: 'g'.repeat(SESSION_TOKEN_LENGTH),
            expected: false,
        },
        {
            name: 'undefined',
            value: undefined,
            expected: false,
        },
        {
            name: 'null',
            value: null,
            expected: false,
        },
    ])('returns $expected when given $name', ({ value, expected }) => {
        expect(isValidToken(value)).toBe(expected);
    });
});

describe('validateSession', () => {
    it('returns noSessionToken error when session cookie is missing', async () => {
        const cookies = { get: vi.fn().mockReturnValue(undefined) };
        const result = validateSession(cookies);

        await expectErrorResult(result, ERRORS.noSessionToken);
        expect(cookies.get).toHaveBeenCalledWith('session');
    });

    it('returns invalidSessionToken error when cookie is invalid', async () => {
        const cookies = { get: vi.fn().mockReturnValue('session-token-123') };
        const result = validateSession(cookies);

        await expectErrorResult(result, ERRORS.invalidSessionToken);
    });

    it('returns token for a valid hex cookie', () => {
        const token = 'a'.repeat(SESSION_TOKEN_LENGTH);
        const cookies = { get: vi.fn().mockReturnValue(token) };
        const result = validateSession(cookies);

        expect(result).toEqual({ data: { token } });
        expect(cookies.get).toHaveBeenCalledWith('session');
    });
});

describe('validateGameLaunchUrl', () => {
    it.each([
        [
            'valid template',
            'mygame://join?host={host}&port={port}&token={token}',
            true,
        ],
        ['missing host', 'mygame://join?port={port}&token={token}', false],
        ['missing port', 'mygame://join?host={host}&token={token}', false],
        ['missing token', 'mygame://join?host={host}&port={port}', false],
        ['non-string', 42, false],
        ['undefined', undefined, false],
        ['null', null, false],
        [
            'invalid url',
            'not a url ?host={host}&port={port}&token={token}',
            false,
        ],
    ])('returns %s', (_name, value, expected) => {
        expect(validateGameLaunchUrl(value)).toBe(expected);
    });
});
