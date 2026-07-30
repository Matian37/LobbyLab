import { json } from '@sveltejs/kit';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from '$lib/constants.js';

export const UNEXPECTED_ERROR_MSG = 'An unexpected error occurred';

export const ERRORS = Object.freeze({
    invalidCredentials: () =>
        json({ msg: 'Invalid login or password' }, { status: 401 }),
    noSessionToken: () =>
        json({ msg: 'No session token provided' }, { status: 401 }),
    invalidSessionToken: () =>
        json({ msg: 'Invalid session token' }, { status: 401 }),
    invalidCredentialTypes: () =>
        json({ msg: 'Login and password must be strings' }, { status: 422 }),
    invalidLoginLength: () =>
        json(
            {
                msg: `Login must be at least ${LOGIN_MIN_LENGTH} and at most ${LOGIN_MAX_LENGTH} characters`,
            },
            { status: 422 }
        ),
    invalidPasswordLength: () =>
        json(
            {
                msg: `Password must be at least ${PASSWORD_MIN_LENGTH} and at most ${PASSWORD_MAX_LENGTH} characters`,
            },
            { status: 422 }
        ),
    loginTaken: () => json({ msg: 'Login is already taken' }, { status: 409 }),
});
