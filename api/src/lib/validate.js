import { ERRORS } from './errors.js';
import {
    LOGIN_MIN_LENGTH,
    PASSWORD_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
    PASSWORD_MAX_LENGTH,
    SESSION_TOKEN_LENGTH,
} from './constants.js';

/**
 * Result of a validation step: either `{ error }` with a SvelteKit `Response`,
 * or `{ data }` with the parsed value.
 *
 * @template T
 * @typedef {{ error: import('@sveltejs/kit').Response } | { data: T }} ValidationResult
 */

/**
 * Parses a request body as a JSON object. Returns an error response when the
 * body is not valid JSON or is not a plain object.
 *
 * @param {Request} request The incoming request.
 * @returns {Promise<ValidationResult<Record<string, unknown>>>} `{ data }` with
 *     the parsed value on success, otherwise `{ error }`.
 */
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

/**
 * Validates a login/password request body against the full credentials schema:
 * both fields present, of string type, and within the configured length limits.
 *
 * @param {Request} request The incoming request.
 * @returns {Promise<ValidationResult<{ login: string, password: string }>>}
 *     `{ data }` with the trimmed credentials on success, otherwise `{ error }`.
 */
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

/**
 * Checks whether a value is a well-formed session token: a string of exactly
 * {@link SESSION_TOKEN_LENGTH} lowercase hexadecimal characters.
 *
 * @param {unknown} str The value to test.
 * @returns {boolean} `true` when the value is a valid token shape.
 */
export function isValidToken(str) {
    return typeof str === 'string' && TOKEN_REGEX.test(str);
}

/**
 * Validates a game launch URL template. The URL must be a string containing the
 * `{host}`, `{port}`, and `{token}` placeholders, and must remain parseable as
 * a URL once the placeholders are substituted with sample values.
 *
 * @param {unknown} url The launch URL template to validate.
 * @returns {boolean} `true` when the URL is shaped correctly.
 */
export function validateGameLaunchUrl(url) {
    if (typeof url !== 'string') return false;

    const required = ['{host}', '{port}', '{token}'];
    if (!required.every((p) => url.includes(p))) return false;

    const testUrl = url
        .replace('{host}', 'localhost')
        .replace('{port}', '8080')
        .replace('{token}', 'token');

    return URL.canParse(testUrl);
}

/**
 * Extracts and validates the session token from the request cookies.
 *
 * @param {import('@sveltejs/kit').Cookies} cookies SvelteKit cookies handle.
 * @returns {ValidationResult<{ token: string }>} `{ data }` with the token when
 *     the `session` cookie is present and well-formed, otherwise `{ error }`.
 */
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
