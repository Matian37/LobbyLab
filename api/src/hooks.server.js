import { httpLogger } from '$lib/logger.js';

export async function handle({ event, resolve }) {
    httpLogger.debug(
        {
            method: event.request.method,
            path: event.url.pathname,
        },
        'request received'
    );

    const start = Date.now();
    const response = await resolve(event);

    const logData = {
        method: event.request.method,
        path: event.url.pathname,
        status: response.status,
        durationMs: Date.now() - start,
    };

    if (httpLogger.level === 'debug') {
        const clone = response.clone();
        const body = await clone.text();
        httpLogger.debug({ ...logData, body }, 'request handled');
    } else {
        httpLogger.info(logData, 'request handled');
    }

    return response;
}
