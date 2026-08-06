import {
    describe,
    it,
    expect,
    beforeAll,
    afterAll,
    beforeEach,
    onTestFinished,
    vi,
} from 'vitest';
import http from 'node:http';
import net from 'node:net';
import crypto from 'node:crypto';
import { once, on } from 'node:events';
import { WebSocket } from 'ws';
import { SERVER_DEFAULT_OPTIONS } from '$lib/constants.js';
import {
    resetSchema,
    setupDatabase,
    teardownDatabase,
} from '$lib/test-database.js';

const OPTIONS = Object.freeze({
    ...SERVER_DEFAULT_OPTIONS,
    queueExtensionIntervalMs: 100,
    queueExtensionMs: 60_000,
    pollIntervalMs: 100,
    pingIntervalMs: 250,
    pongTimeoutMs: 500,
    maxMissedPongs: 2,
});

let db, serverModule, helperSql, container;

beforeAll(async () => {
    ({ container, helperSql } = await setupDatabase());
    db = await import('$lib/db.js');
    serverModule = await import('$lib/server.js');
}, 60000);

beforeEach(async () => {
    await resetSchema(helperSql);
});

afterAll(async () => {
    await teardownDatabase({ container, helperSql, sqlPool: db?.sql });
}, 30000);

const WAIT = { timeout: 10_000, interval: 25 };
const TEST_TIMEOUT = 30_000;

async function startServer() {
    const httpServer = http.createServer();
    const connectionServer = await serverModule.createWebSocketServer(
        httpServer,
        OPTIONS
    );

    httpServer.listen(0, '127.0.0.1');
    await once(httpServer, 'listening');

    const { port } = httpServer.address();
    const url = `ws://127.0.0.1:${port}${OPTIONS.connectionPath}`;

    onTestFinished(() => {
        connectionServer.close();

        return new Promise((resolve) => {
            const timer = setTimeout(resolve, 2000);
            httpServer.close(() => {
                clearTimeout(timer);
                resolve();
            });
        });
    });

    return { httpServer, connectionServer, url, port };
}

function connectClient(url, token, { autoPong = true } = {}) {
    const client = new WebSocket(url, {
        headers: { cookie: `session=${token}` },
        autoPong,
    });
    client.on('error', () => {});
    onTestFinished(() => client.terminate());
    return client;
}

async function onceMessage(client) {
    const [data] = await once(client, 'message', {
        signal: AbortSignal.timeout(10_000),
    });
    return JSON.parse(data.toString());
}

async function onceClose(client) {
    const [code, reason] = await once(client, 'close', {
        signal: AbortSignal.timeout(10_000),
    });
    return { code, reason: reason.toString() };
}

// Opens a raw TCP socket that completes the WebSocket handshake but then
// goes silent, so the server can only detect it via missed pongs.
async function openDeadSocket(port, token) {
    const socket = net.connect(port, '127.0.0.1');
    onTestFinished(() => socket.destroy());

    const signal = AbortSignal.timeout(10_000);

    await once(socket, 'connect', { signal });

    const key = crypto.randomBytes(16).toString('base64');
    socket.write(
        `GET /api/connection HTTP/1.1\r\n` +
            `Host: 127.0.0.1:${port}\r\n` +
            `Upgrade: websocket\r\n` +
            `Connection: Upgrade\r\n` +
            `Sec-WebSocket-Key: ${key}\r\n` +
            `Sec-WebSocket-Version: 13\r\n` +
            `Cookie: session=${token}\r\n\r\n`
    );

    for await (const [chunk] of on(socket, 'data', { signal })) {
        if (chunk.includes('101 Switching Protocols')) break;
    }

    return socket;
}

async function isUserWaiting(login) {
    const rows = await helperSql`
        SELECT match_id IS NULL AND queued_until > NOW() as waiting
        FROM users
        WHERE login = ${login}
    `;
    return rows[0].waiting === true;
}

async function getUserWebsocket(login) {
    const rows = await helperSql`
        SELECT last_websocket_id FROM users WHERE login = ${login}
    `;
    return Number(rows[0].last_websocket_id);
}

async function assignMatch(login, { host = 'host', port = 'port' } = {}) {
    await helperSql`
        INSERT INTO matches (id, host, port) VALUES (1, ${host}, ${port})
    `;
    await helperSql`
        UPDATE users
        SET match_id = 1, match_auth_token = 'TOKEN'
        WHERE login = ${login}
    `;
}

describe('matchmaking', () => {
    it(
        'matchmakes a queued user and sends match details',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            const { connectionServer, url } = await startServer();
            const client = connectClient(url, token);

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    1
                );
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);

            const message = onceMessage(client);
            const close = onceClose(client);

            await assignMatch('user1');

            await expect(message).resolves.toEqual({
                login: 'user1',
                host: 'host',
                port: 'port',
                matchAuthToken: 'TOKEN',
            });
            expect(await isUserWaiting('user1')).toBe(false);
            await expect(close).resolves.toMatchObject({ code: 1000 });

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    0
                );
            }, WAIT);
        },
        TEST_TIMEOUT
    );

    it(
        'disconnects a user after missed pongs',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            const { connectionServer, url } = await startServer();
            const client = connectClient(url, token, { autoPong: false });
            const close = onceClose(client);

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    1
                );
            }, WAIT);
            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    0
                );
            }, WAIT);

            await expect(close).resolves.toMatchObject({ code: 1006 });

            expect(await isUserWaiting('user1')).toBe(false);
        },
        TEST_TIMEOUT
    );

    it(
        'closes connection when user is already matched',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            await assignMatch('user1');

            const { connectionServer, url } = await startServer();
            const client = connectClient(url, token);
            const close = onceClose(client);

            await expect(close).resolves.toMatchObject({
                code: 4000,
                reason: 'Already in match',
            });

            expect(connectionServer.connections.getQueued()).toHaveLength(0);
            expect(await isUserWaiting('user1')).toBe(false);
        },
        TEST_TIMEOUT
    );

    it(
        'replaces a connection when user reconnects',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            const { connectionServer, url } = await startServer();

            const client1 = connectClient(url, token);
            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    1
                );
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(1);

            const close1 = onceClose(client1);
            const client2 = connectClient(url, token);

            await vi.waitFor(() => {
                const queued = connectionServer.connections.getQueued();
                expect(queued).toHaveLength(1);
                expect(queued[0].login).toBe('user1');
                expect(queued[0].websocketId).toBe(2);
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(2);

            await expect(close1).resolves.toMatchObject({
                code: 4001,
                reason: 'Replaced by new connection',
            });

            const message2 = onceMessage(client2);
            await assignMatch('user1');
            await expect(message2).resolves.toMatchObject({
                login: 'user1',
                host: 'host',
                port: 'port',
            });
            expect(await isUserWaiting('user1')).toBe(false);
        },
        TEST_TIMEOUT
    );

    it(
        'replaces a connection across api instances',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');

            const srvA = await startServer();
            const srvB = await startServer();

            const clientA = connectClient(srvA.url, token);
            await vi.waitFor(() => {
                expect(
                    srvA.connectionServer.connections.getQueued()
                ).toHaveLength(1);
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(1);

            const closeA = onceClose(clientA);
            connectClient(srvB.url, token);
            await vi.waitFor(() => {
                expect(
                    srvB.connectionServer.connections.getQueued()
                ).toHaveLength(1);
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(2);

            await expect(closeA).resolves.toMatchObject({
                code: 4001,
                reason: 'Replaced by new connection',
            });

            await vi.waitFor(() => {
                expect(
                    srvA.connectionServer.connections.getQueued()
                ).toHaveLength(0);
            }, WAIT);
            expect(srvB.connectionServer.connections.getQueued()).toHaveLength(
                1
            );
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(2);
        },
        TEST_TIMEOUT
    );

    it(
        'removes a connection on graceful disconnect',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            const { connectionServer, url } = await startServer();
            const client = connectClient(url, token);

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    1
                );
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(true);
            expect(await getUserWebsocket('user1')).toBe(1);

            client.close(1000, 'bye');

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    0
                );
            }, WAIT);
            expect(await isUserWaiting('user1')).toBe(false);
        },
        TEST_TIMEOUT
    );

    it(
        'fails a dead connection after missed pongs',
        async () => {
            await db.addUser('user1', 'pass');
            const token = await db.addSession('user1');
            const { connectionServer, port } = await startServer();

            const dead = await openDeadSocket(port, token);

            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    1
                );
            }, WAIT);
            await vi.waitFor(() => {
                expect(connectionServer.connections.getQueued()).toHaveLength(
                    0
                );
            }, WAIT);

            if (!dead.destroyed) {
                await once(dead, 'close', {
                    signal: AbortSignal.timeout(2000),
                }).catch(() => {});
            }

            expect(await isUserWaiting('user1')).toBe(false);
        },
        TEST_TIMEOUT
    );
});
