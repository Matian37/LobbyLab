import { json } from '@sveltejs/kit';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from './constants.js';

export const UNEXPECTED_ERROR_MSG = 'An unexpected error occurred';

// TODO: use func instead of json syntax
export const ERRORS = Object.freeze({
    invalidJSON: () => json({ msg: 'Invalid JSON' }, { status: 400 }),
    missingLogin: () => json({ msg: 'Login is missing' }, { status: 422 }),
    missingPassword: () =>
        json({ msg: 'Password is missing' }, { status: 422 }),
    invalidLoginType: () =>
        json({ msg: 'Login must be a string' }, { status: 422 }),
    invalidPasswordType: () =>
        json({ msg: 'Password must be a string' }, { status: 422 }),
    invalidCredentials: () =>
        json({ msg: 'Invalid login or password' }, { status: 401 }),
    noSessionToken: () =>
        json({ msg: 'No session token provided' }, { status: 401 }),
    invalidSessionToken: () =>
        json({ msg: 'Invalid session token' }, { status: 401 }),
    invalidLoginLength: () =>
        json(
            {
                msg: `Login must be at least ${LOGIN_MIN_LENGTH} and at most ${LOGIN_MAX_LENGTH} characters long`,
            },
            { status: 422 }
        ),
    invalidPasswordLength: () =>
        json(
            {
                msg: `Password must be at least ${PASSWORD_MIN_LENGTH} and at most ${PASSWORD_MAX_LENGTH} characters long`,
            },
            { status: 422 }
        ),
    loginTaken: () => json({ msg: 'Login is already taken' }, { status: 409 }),
    wsRequired: () => json({ msg: 'Websocket is required' }, { status: 426 }),
});

export class ConnectionStateError extends Error {
    constructor(message) {
        super(message);
        this.name = 'ConnectionStateError';
    }
}

export const CONNECTION_ERRORS = Object.freeze({
    cannotOpenConnection: (state) =>
        new ConnectionStateError(`Cannot open a connection in state ${state}`),
    cannotOpenServer: (state) =>
        new ConnectionStateError(
            `Cannot open a connection server in state ${state}`
        ),
    alreadyAttached: () =>
        new ConnectionStateError(
            'Connection server is already attached to an http server'
        ),
    notAttached: () =>
        new ConnectionStateError(
            'Connection server is not attached to an http server'
        ),
});
