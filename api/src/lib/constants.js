/**
 * Minimum and maximum number of characters accepted for a password.
 */
export const PASSWORD_MIN_LENGTH = 5;
export const PASSWORD_MAX_LENGTH = 64;

/**
 * Minimum and maximum number of characters accepted for a login.
 */
export const LOGIN_MIN_LENGTH = 3;
export const LOGIN_MAX_LENGTH = 20;

/**
 * Length in characters of a session token. Tokens are generated as
 * `SESSION_TOKEN_LENGTH / 2` random bytes encoded as lowercase hex.
 */
export const SESSION_TOKEN_LENGTH = 64;

/**
 * Lifecycle state used for connection-like classes.
 *
 * @readonly
 * @enum {string}
 */
export const State = Object.freeze({
    INIT: 'INIT',
    OPEN: 'OPEN',
    CLOSED: 'CLOSED',
});

/**
 * Status of the matchmaking button shown to the user on the home page.
 *
 * @readonly
 * @enum {string}
 */
export const WaitingStatus = Object.freeze({
    NOT_ACTIVE: 'not active',
    PENDING: 'pending',
    FOUND: 'found',
});

/**
 * Options that tune the ping/pong watchdog of a connection.
 *
 * @typedef {Object} ConnectionOptions
 * @property {number} pingIntervalMs How often a ping control frame is sent.
 * @property {number} pongTimeoutMs How long to wait for a pong before it is counted as missed.
 * @property {number} maxMissedPongs Consecutive missed pongs tolerated before the connection is hard-closed.
 */

/**
 * The default per-connection watchdog options.
 *
 * @type {Readonly<ConnectionOptions>}
 */
export const CONNECTION_DEFAULT_OPTIONS = Object.freeze({
    pingIntervalMs: 5_000,
    pongTimeoutMs: 3_000,
    maxMissedPongs: 2,
});

/**
 * Default options used when constructing a connection server.
 *
 * @typedef {ConnectionOptions & {
 *     connectionPath: string,
 *     queueExtensionIntervalMs: number,
 *     queueExtensionMs: number,
 *     pollIntervalMs: number
 * }} ServerOptions
 */

/**
 * Default per-server options. Extends the connection defaults with the
 * WebSocket path and the scheduling intervals of the queue extension and
 * connection-status poll loops.
 *
 * @type {Readonly<ServerOptions>}
 */
export const SERVER_DEFAULT_OPTIONS = Object.freeze({
    ...CONNECTION_DEFAULT_OPTIONS,
    connectionPath: '/api/connection',
    queueExtensionIntervalMs: 3_000,
    queueExtensionMs: 5_000,
    pollIntervalMs: 2_500,
});
