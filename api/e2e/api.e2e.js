import { test, expect } from './fixtures.js';
import { once } from 'node:events';
import { WebSocket } from 'ws';
import {
    BASE_URL,
    createMatchResult,
    addClientFile,
    removeClientFile,
    CLIENT_FILE,
    CLIENT_CONTENT,
} from './helpers.js';

const WS_URL = () => BASE_URL.replace(/^http/, 'ws') + '/api/connection';

function openWs(token, { autoPong = true } = {}) {
    const options = { autoPong };
    if (token !== null) options.headers = { cookie: `session=${token}` };
    return new WebSocket(WS_URL(), options);
}

async function onceClose(ws, timeout = 10_000) {
    const [code, reason] = await once(ws, 'close', {
        signal: AbortSignal.timeout(timeout),
    });
    return { code, reason: reason.toString() };
}

async function onceMessage(ws, timeout = 10_000) {
    const [data] = await once(ws, 'message', {
        signal: AbortSignal.timeout(timeout),
    });
    return JSON.parse(data.toString());
}

async function isWaiting(sql, login) {
    const rows = await sql`
        SELECT match_id IS NULL AND queued_until > NOW() AS waiting
        FROM users
        WHERE login = ${login}
    `;
    return rows[0]?.waiting === true;
}

async function getToken(sql, login) {
    const rows = await sql`
        SELECT token FROM sessions WHERE login = ${login}
    `;
    return rows[0].token;
}

test.describe('register', () => {
    test('registers a new user and sets a session cookie', async ({
        request,
    }) => {
        const response = await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        expect(response.status()).toBe(200);
        expect(await response.json()).toEqual({});

        const session = await request.get(`${BASE_URL}/api/session`);
        expect(await session.json()).toEqual({ login: 'login' });
    });

    test('rejects invalid JSON', async ({ request }) => {
        const response = await request.post(`${BASE_URL}/api/register`, {
            data: 'not json',
        });

        expect(response.status()).toBe(400);
    });

    test('rejects malformed credentials', async ({ request }) => {
        const response = await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'a', password: 'password' },
        });

        expect(response.status()).toBe(422);
    });

    test('rejects a duplicate login', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const response = await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        expect(response.status()).toBe(409);
    });
});

test.describe('login', () => {
    test('logs in with correct credentials', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.post(`${BASE_URL}/api/login`, {
            data: { login: 'login', password: 'password' },
        });

        expect(response.status()).toBe(200);
    });

    test('rejects wrong credentials', async ({ request }) => {
        const response = await request.post(`${BASE_URL}/api/login`, {
            data: { login: 'login', password: 'wrong' },
        });

        expect(response.status()).toBe(401);
    });
});

test.describe('logout', () => {
    test('clears the session', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.post(`${BASE_URL}/api/logout`);
        expect(response.status()).toBe(200);

        const session = await request.get(`${BASE_URL}/api/session`);
        expect(session.status()).toBe(401);
    });

    test('rejects without a session', async ({ request }) => {
        const response = await request.post(`${BASE_URL}/api/logout`);

        expect(response.status()).toBe(401);
    });
});

test.describe('session', () => {
    test('returns the login', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.get(`${BASE_URL}/api/session`);

        expect(response.status()).toBe(200);
        expect(await response.json()).toEqual({ login: 'login' });
    });

    test('returns 401 without a session', async ({ request }) => {
        const response = await request.get(`${BASE_URL}/api/session`);

        expect(response.status()).toBe(401);
    });

    test('returns 401 for an invalid session token', async ({ playwright }) => {
        const context = await playwright.request.newContext({
            extraHTTPHeaders: { Cookie: 'session=garbage' },
        });

        try {
            const response = await context.get(`${BASE_URL}/api/session`);
            expect(response.status()).toBe(401);
        } finally {
            await context.dispose();
        }
    });
});

test.describe('match', () => {
    test('returns null without a match', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.get(`${BASE_URL}/api/match`);

        expect(await response.json()).toEqual({ match: null });
    });

    test('returns the assigned match', async ({ request, helperSql }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        await createMatchResult(helperSql, ['login']);

        const response = await request.get(`${BASE_URL}/api/match`);

        expect(await response.json()).toEqual({
            match: {
                host: 'game-host',
                port: '7777',
                matchAuthToken: expect.any(String),
            },
        });
    });

    test('returns 401 without a session', async ({ request }) => {
        const response = await request.get(`${BASE_URL}/api/match`);

        expect(response.status()).toBe(401);
    });

    test('returns 401 for an invalid session token', async ({ playwright }) => {
        const context = await playwright.request.newContext({
            extraHTTPHeaders: { Cookie: 'session=garbage' },
        });

        try {
            const response = await context.get(`${BASE_URL}/api/match`);

            expect(response.status()).toBe(401);
        } finally {
            await context.dispose();
        }
    });
});

test.describe('results', () => {
    test('returns no matches for a new user', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.get(`${BASE_URL}/api/results`);

        expect(await response.json()).toEqual({ matches: [] });
    });

    test('returns the user matches', async ({ request, helperSql }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        await createMatchResult(helperSql, ['login'], {
            players: ['login', 'other'],
            winner: 'login',
        });

        const response = await request.get(`${BASE_URL}/api/results`);

        expect(await response.json()).toEqual({
            matches: [
                {
                    id: expect.any(Number),
                    details: { players: ['login', 'other'], winner: 'login' },
                    canceled: false,
                    active: false,
                },
            ],
        });
    });

    test('returns 401 without a session', async ({ request }) => {
        const response = await request.get(`${BASE_URL}/api/results`);

        expect(response.status()).toBe(401);
    });

    test('returns 401 for an invalid session token', async ({ playwright }) => {
        const context = await playwright.request.newContext({
            extraHTTPHeaders: { Cookie: 'session=garbage' },
        });

        try {
            const response = await context.get(`${BASE_URL}/api/results`);

            expect(response.status()).toBe(401);
        } finally {
            await context.dispose();
        }
    });
});

test.describe('download', () => {
    test.afterEach(removeClientFile);

    test('returns 401 without a session', async ({ request }) => {
        const response = await request.get(`${BASE_URL}/api/download`);

        expect(response.status()).toBe(401);
    });

    test('returns 401 for an invalid session token', async ({ playwright }) => {
        const context = await playwright.request.newContext({
            extraHTTPHeaders: { Cookie: 'session=garbage' },
        });

        try {
            const response = await context.get(`${BASE_URL}/api/download`);

            expect(response.status()).toBe(401);
        } finally {
            await context.dispose();
        }
    });

    test('returns 404 when the client file is missing', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });

        const response = await request.get(`${BASE_URL}/api/download`);

        expect(response.status()).toBe(404);
    });

    test('downloads the client file', async ({ request }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        await addClientFile();

        const response = await request.get(`${BASE_URL}/api/download`);

        expect(response.status()).toBe(200);
        expect(response.headers()['content-disposition']).toBe(
            `attachment; filename="${CLIENT_FILE}"`
        );
        expect(await response.text()).toBe(CLIENT_CONTENT);
    });
});

test.describe('connection', () => {
    test('rejects a plain HTTP request', async ({ request }) => {
        const response = await request.get(`${BASE_URL}/api/connection`);

        expect(response.status()).toBe(426);
    });

    test('rejects a connection without a session cookie', async () => {
        const ws = openWs(null);
        try {
            const close = await onceClose(ws);
            expect(close).toEqual({ code: 4401, reason: 'No cookie' });
        } finally {
            ws.terminate();
        }
    });

    test('rejects a connection with an invalid session token', async () => {
        const ws = openWs('garbage');
        try {
            const close = await onceClose(ws);
            expect(close).toEqual({
                code: 4401,
                reason: 'Invalid session token',
            });
        } finally {
            ws.terminate();
        }
    });

    test('terminates a connection that misses pongs', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');

        const ws = openWs(token, { autoPong: false });
        try {
            const close = await onceClose(ws, 30_000);
            expect(close.code).toBe(1006);
        } finally {
            ws.terminate();
        }
    });

    test('waits and receives the match details', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');

        const ws = openWs(token);
        try {
            await expect
                .poll(() => isWaiting(helperSql, 'login'), {
                    timeout: 10_000,
                })
                .toBe(true);

            const messagePromise = onceMessage(ws);
            const closePromise = onceClose(ws);
            await createMatchResult(helperSql, ['login']);

            const message = await messagePromise;
            expect(message).toEqual({
                login: 'login',
                host: 'game-host',
                port: '7777',
                matchAuthToken: expect.any(String),
            });

            const close = await closePromise;
            expect(close.code).toBe(1000);
        } finally {
            ws.terminate();
        }
    });

    test('replaces a connection in the same instance', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');

        const firstWs = openWs(token);
        let secondWs;
        try {
            await expect
                .poll(() => isWaiting(helperSql, 'login'), {
                    timeout: 10_000,
                })
                .toBe(true);

            const closeFirst = onceClose(firstWs);
            secondWs = openWs(token);

            const close = await closeFirst;
            expect(close).toEqual({
                code: 4001,
                reason: 'Replaced by new connection',
            });
        } finally {
            secondWs?.terminate();
            firstWs.terminate();
        }
    });

    test('replaces a connection when a newer one registers in another instance', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');

        const ws = openWs(token);
        try {
            await expect
                .poll(() => isWaiting(helperSql, 'login'), {
                    timeout: 10_000,
                })
                .toBe(true);

            await helperSql`
                UPDATE users
                SET last_websocket_id = last_websocket_id + 1
                WHERE login = 'login'
            `;

            const close = await onceClose(ws);
            expect(close).toEqual({
                code: 4001,
                reason: 'Replaced by new connection',
            });
        } finally {
            ws.terminate();
        }
    });

    test('closes the connection when the user already has a match', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');
        await createMatchResult(helperSql, ['login']);

        const ws = openWs(token);
        try {
            const close = await onceClose(ws);
            expect(close).toEqual({ code: 4000, reason: 'Already in match' });
        } finally {
            ws.terminate();
        }
    });

    test('removes the queue entry when the connection is closed', async ({
        request,
        helperSql,
    }) => {
        await request.post(`${BASE_URL}/api/register`, {
            data: { login: 'login', password: 'password' },
        });
        const token = await getToken(helperSql, 'login');

        const ws = openWs(token);
        try {
            await expect
                .poll(() => isWaiting(helperSql, 'login'), {
                    timeout: 10_000,
                })
                .toBe(true);

            ws.close(1000, 'bye');

            await expect
                .poll(async () => !(await isWaiting(helperSql, 'login')), {
                    timeout: 10_000,
                })
                .toBe(true);
        } finally {
            ws.terminate();
        }
    });
});
