export const PASSWORD_MIN_LENGTH = 5;
export const PASSWORD_MAX_LENGTH = 64;

export const LOGIN_MIN_LENGTH = 3;
export const LOGIN_MAX_LENGTH = 20;

export const SESSION_TOKEN_LENGTH = 64;

export const State = Object.freeze({
    INIT: 'INIT',
    OPEN: 'OPEN',
    CLOSED: 'CLOSED',
});

export const CONNECTION_DEFAULT_OPTIONS = Object.freeze({
    pingIntervalMs: 5_000,
    pongTimeoutMs: 3_000,
    maxMissedPongs: 2,
});

export const SERVER_DEFAULT_OPTIONS = Object.freeze({
    ...CONNECTION_DEFAULT_OPTIONS,
    connectionPath: '/api/connection',
    queueExtensionIntervalMs: 3_000,
    queueExtensionMs: 5_000,
    pollIntervalMs: 2_500,
});
