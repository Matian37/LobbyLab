import { json } from '@sveltejs/kit';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from './constants.js';

/**
 * Fallback message shown to the user when a request fails unexpectedly and no
 * more specific error is available.
 */
export const UNEXPECTED_ERROR_MSG = 'An unexpected error occurred';

/**
 * A pre-built SvelteKit JSON response for a given HTTP error. Each factory
 * returns a fresh `Response` so callers can return it directly from a route
 * handler.
 *
 * @typedef {() => import('@sveltejs/kit').Response} ErrorFactory
 */

/**
 * Factories for the standard REST error responses used across the API. Every
 * factory returns a `{ "msg": string }` JSON body with the corresponding HTTP
 * status code.
 *
 * @type {{ [name: string]: ErrorFactory }}
 */
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
    notFound: () => json({ msg: 'Not found' }, { status: 404 }),
});

/**
 * Error thrown when a connection or the connection server is used in an
 * invalid lifecycle state (for example calling `open()` twice).
 */
export class ConnectionStateError extends Error {
    /**
     * @param {string} message Human-readable description of the invalid state.
     */
    constructor(message) {
        super(message);
        this.name = 'ConnectionStateError';
    }
}

/**
 * Factories for {@link ConnectionStateError} instances raised by the
 * WebSocket connection/server lifecycle.
 *
 * @type {{ [name: string]: (...args: any[]) => ConnectionStateError }}
 */
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
