import pino from 'pino';

/**
 * Logging verbosity, read from the `LOG_LEVEL` environment variable. When set
 * to `'debug'` the output is pretty-printed for developer ergonomics.
 */
const level = process.env.LOG_LEVEL ?? 'info';

/**
 * Root pino logger configured with the effective level and additional pretty-printing.
 * @type {import('pino').Logger}
 */
const logger = pino({
    level,
    ...(level === 'debug'
        ? { transport: { target: 'pino-pretty', options: { colorize: true } } }
        : {}),
});

/**
 * Logger for HTTP request lifecycle messages. Bound to `component: 'http'`.
 */
export const httpLogger = logger.child({ component: 'http' });
/**
 * Logger for WebSocket connection/server messages. Bound to
 * `component: 'websocket'`.
 */
export const websocketLogger = logger.child({ component: 'websocket' });
/**
 * Logger for database driver messages. Bound to `component: 'database'`.
 */
export const databaseLogger = logger.child({ component: 'database' });
