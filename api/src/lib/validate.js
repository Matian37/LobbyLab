import { ERRORS } from './errors.js';
import {
    LOGIN_MIN_LENGTH,
    PASSWORD_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
    PASSWORD_MAX_LENGTH,
    SESSION_TOKEN_LENGTH,
} from './constants.js';

export async function validateRequest(request) {
    let body;
    try {
        body = await request.json();
    } catch {
        return { error: ERRORS.invalidJSON() };
    }

    if (typeof body !== 'object' || body === null || Array.isArray(body)) {
        return { error: ERRORS.invalidJSON() };
    }

    return { data: body };
}

export async function validateCredentialsSchema(request) {
    const result = await validateRequest(request);
    if (result.error !== undefined) return { error: result.error };

    const body = result.data;

    if (body.login === undefined) {
        return { error: ERRORS.missingLogin() };
    }
    if (body.password === undefined) {
        return { error: ERRORS.missingPassword() };
    }

    if (typeof body.login !== 'string') {
        return { error: ERRORS.invalidLoginType() };
    }
    if (typeof body.password !== 'string') {
        return { error: ERRORS.invalidPasswordType() };
    }

    if (
        body.login.length < LOGIN_MIN_LENGTH ||
        body.login.length > LOGIN_MAX_LENGTH
    ) {
        return { error: ERRORS.invalidLoginLength() };
    }
    if (
        body.password.length < PASSWORD_MIN_LENGTH ||
        body.password.length > PASSWORD_MAX_LENGTH
    ) {
        return { error: ERRORS.invalidPasswordLength() };
    }

    return { data: { login: body.login, password: body.password } };
}

const TOKEN_REGEX = new RegExp(`^[0-9a-f]{${SESSION_TOKEN_LENGTH}}$`);

export function isValidToken(str) {
    return typeof str === 'string' && TOKEN_REGEX.test(str);
}

export function validateSession(cookies) {
    const cookie = cookies.get('session');
    if (cookie === undefined) {
        return { error: ERRORS.noSessionToken() };
    }
    if (!isValidToken(cookie)) {
        return { error: ERRORS.invalidSessionToken() };
    }
    return { data: { token: cookie } };
}
