import pino from 'pino';

const level = process.env.LOG_LEVEL ?? 'info';

const logger = pino({
    level,
    ...(level === 'debug'
        ? { transport: { target: 'pino-pretty', options: { colorize: true } } }
        : {}),
});

export const httpLogger = logger.child({ component: 'http' });
export const websocketLogger = logger.child({ component: 'websocket' });
export const databaseLogger = logger.child({ component: 'database' });
