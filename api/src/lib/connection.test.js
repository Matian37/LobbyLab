import { vi, it, expect, describe, beforeEach, afterEach } from 'vitest';
import {
    Connection,
    ConnectionServer,
    DEFAULT_OPTIONS,
    State,
} from '$lib/connection.js';
import * as db from '$lib/db.js';

vi.mock('$lib/db.js', () => ({
    getLoginFromToken: vi.fn(),
    extendQueueStatuses: vi.fn(),
    removeQueueStatus: vi.fn(),
    setUserWebsocket: vi.fn(),
    getConnectionStatuses: vi.fn(),
}));

vi.mock('ws', () => {
    const OPEN = 1;
    const CLOSING = 2;
    const CLOSED = 3;

    return {
        WebSocket: { OPEN, CLOSING, CLOSED },
        WebSocketServer: class WebSocketServer {
            on() {}
        },
    };
});

class FakeWebSocket {
    constructor() {
        this.listeners = new Map();
        this.pings = 0;
        this.pingPayloads = [];
        this.latestPingPayload = null;
        this.sent = [];
        this.closed = false;
        this.terminated = false;
        this.readyState = 1;
        this.closeCode = undefined;
    }

    on(event, callback) {
        this.listeners.set(event, callback);
    }

    off(event, callback) {
        if (this.listeners.get(event) === callback) {
            this.listeners.delete(event);
        }
    }

    emit(event, ...args) {
        this.listeners.get(event)?.apply(this, args);
    }

    ping(data) {
        this.pings += 1;
        this.pingPayloads.push(data);
        this.latestPingPayload = data;
    }

    send(data) {
        this.sent.push(data);
    }

    close(code) {
        if (this.closed) return;
        this.closed = true;
        this.closeCode = code;
        this.readyState = 3;
        this.emit('close', { code });
    }

    terminate() {
        this.closed = true;
        this.terminated = true;
        this.readyState = 3;
        this.emit('close', { code: 1006 });
    }
}

describe('Connection', () => {
    beforeEach(() => {
        vi.useFakeTimers();
        vi.clearAllMocks();
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('is inactive before it starts', () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        expect(connection.state).toBe(State.INIT);
    });

    it('sends a ping periodically', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.pings).toBe(1);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.pings).toBe(2);
    });

    it('advances the ping sequence after a pong is received', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        const firstSeq = ws.latestPingPayload;

        ws.emit('pong', firstSeq);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.latestPingPayload).not.toBe(firstSeq);
    });

    it('closes the connection when a ping fails', async () => {
        const ws = new FakeWebSocket();
        ws.ping = () => {
            throw new Error('connection closing');
        };
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);

        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
    });

    it('closes when the websocket is not open on start', async () => {
        const ws = new FakeWebSocket();
        ws.readyState = 3;
        const connection = new Connection(ws, 'user1');
        const onClose = vi.fn();
        connection.on('close', onClose);

        connection.open();

        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('does not start when already closed', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.close();
        connection.open();

        expect(connection.state).toBe(State.CLOSED);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.pings).toBe(0);
    });

    it('counts a missed pong when no pong is received in time', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);

        expect(connection.missedPongs).toBe(1);
        expect(ws.terminated).toBe(false);
    });

    it('resets the missed pong counter when a pong for the latest ping is received', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);
        expect(connection.missedPongs).toBe(1);

        await vi.advanceTimersByTimeAsync(
            DEFAULT_OPTIONS.pingIntervalMs - DEFAULT_OPTIONS.pongTimeoutMs
        );
        expect(ws.pings).toBe(2);

        ws.emit('pong', ws.latestPingPayload);

        expect(connection.missedPongs).toBe(0);
    });

    it('ignores a stale pong that answers an older ping', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);
        expect(connection.missedPongs).toBe(1);

        await vi.advanceTimersByTimeAsync(
            DEFAULT_OPTIONS.pingIntervalMs - DEFAULT_OPTIONS.pongTimeoutMs
        );
        expect(ws.pings).toBe(2);

        ws.emit('pong', ws.pingPayloads[0]);

        expect(connection.missedPongs).toBe(1);
        expect(connection.state).toBe(State.OPEN);
    });

    it('ignores a stale timeout callback after the pong was received', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        const answeredSeq = ws.latestPingPayload;

        ws.emit('pong', answeredSeq);
        connection.onPongMiss(answeredSeq);

        expect(connection.missedPongs).toBe(0);
        expect(connection.state).toBe(State.OPEN);
        expect(ws.terminated).toBe(false);
    });

    it('ignores a pong with no payload', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        ws.emit('pong');

        expect(connection.missedPongs).toBe(0);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);

        expect(connection.missedPongs).toBe(1);
    });

    it('does not terminate the connection before reaching the missed pong limit', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);

        expect(ws.terminated).toBe(false);
    });

    it('terminates the connection after the max amount of missed pongs', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        for (let i = 0; i < DEFAULT_OPTIONS.maxMissedPongs + 1; i++) {
            await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
            await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pongTimeoutMs);
        }

        expect(connection.missedPongs).toBe(DEFAULT_OPTIONS.maxMissedPongs + 1);
        expect(ws.terminated).toBe(true);
    });

    it('sends the match payload with the auth token and closes the connection', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();
        connection.sendMatchAndClose({
            login: 'user1',
            host: 'h',
            port: 'p',
            matchAuthToken: 'TOKEN',
        });

        expect(ws.sent).toEqual([
            JSON.stringify({
                login: 'user1',
                host: 'h',
                port: 'p',
                matchAuthToken: 'TOKEN',
            }),
        ]);
        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
    });

    it('does not send the match payload when already closed', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();
        connection.close();
        connection.sendMatchAndClose({ login: 'user1', host: 'h' });

        expect(ws.sent).toEqual([]);
    });

    it('closes the connection when the socket is not open', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();
        ws.readyState = 3;
        connection.sendMatchAndClose({ login: 'user1', host: 'h' });

        expect(ws.sent).toEqual([]);
        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
    });

    it('cleans up when the client closes the connection', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();

        ws.emit('close', { code: 1000 });

        expect(connection.state).toBe(State.CLOSED);
        expect(connection.pingTimer).toBe(null);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs * 5);
        expect(ws.pings).toBe(0);
    });

    it('does not ping after the connection is closed', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();
        connection.close();

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs * 3);

        expect(ws.pings).toBe(0);
        expect(ws.closed).toBe(true);
    });

    it('is safe to close multiple times', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.open();
        connection.close();
        connection.close();

        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
    });

    it('closes without start', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');
        const onClose = vi.fn();
        connection.on('close', onClose);

        connection.close();

        expect(connection.state).toBe(State.CLOSED);
        expect(ws.closed).toBe(true);
        expect(onClose).toHaveBeenCalledTimes(1);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs * 3);
        expect(ws.pings).toBe(0);
    });

    it('does not start after closing without start', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');

        connection.close();
        connection.open();

        expect(connection.state).toBe(State.CLOSED);
        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.pings).toBe(0);
    });

    it('emits a close event once when closed', async () => {
        const ws = new FakeWebSocket();
        const connection = new Connection(ws, 'user1');
        const onClose = vi.fn();
        connection.on('close', onClose);

        connection.open();
        connection.close();
        connection.close();

        expect(onClose).toHaveBeenCalledTimes(1);
        expect(ws.closed).toBe(true);
    });
});

describe('ConnectionServer', () => {
    let lastWebsocketIds;
    const VALID_TOKEN = 'a'.repeat(64);

    async function createServer(wss) {
        const server = new ConnectionServer(wss);
        await server.open();
        return server;
    }

    beforeEach(() => {
        vi.useFakeTimers();
        vi.clearAllMocks();
        lastWebsocketIds = new Map();
        db.removeQueueStatus.mockResolvedValue(true);
        db.setUserWebsocket.mockImplementation(async (login, websocketId) => {
            lastWebsocketIds.set(login, websocketId);
            return true;
        });
        db.getConnectionStatuses.mockImplementation(
            async (logins) =>
                new Map(
                    logins.map((login) => [
                        login,
                        {
                            websocketId: lastWebsocketIds.get(login) ?? null,
                            matchId: null,
                            matchAuthToken: null,
                            host: null,
                            port: null,
                        },
                    ])
                )
        );
        db.extendQueueStatuses.mockImplementation(
            async (connections) =>
                new Set(connections.map((connection) => connection.login))
        );
    });

    afterEach(() => {
        vi.useRealTimers();
    });

    it('polls the connection statuses periodically', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);

        expect(db.getConnectionStatuses).toHaveBeenCalledWith(['user1']);
        expect(ws.closed).toBe(false);
    });

    it('rejects the connection when the session cookie is missing', async () => {
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, { headers: {} });

        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4401);
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
    });

    it('rejects a session token with an invalid format', async () => {
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: 'session=TOKEN' },
        });

        expect(db.getLoginFromToken).not.toHaveBeenCalled();
        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4401);
    });

    it('rejects a session token with a valid format that does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue(null);
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(db.getLoginFromToken).toHaveBeenCalledWith(VALID_TOKEN);
        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4401);
    });

    it('closes the connection when authentication fails', async () => {
        db.getLoginFromToken.mockRejectedValue(new Error('db down'));
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(1011);
    });

    it('starts a connection for a valid session', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(server.connections.has('user1')).toBe(true);
        expect(ws.closed).toBe(false);

        const connection = server.connections.get('user1');
        expect(db.setUserWebsocket).toHaveBeenCalledWith(
            'user1',
            connection.websocketId,
            5000
        );

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pingIntervalMs);
        expect(ws.pings).toBe(1);
    });

    it('rejects the connection when the user is already in a match', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        db.setUserWebsocket.mockResolvedValue(false);
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4000);
        expect(server.connections.has('user1')).toBe(false);
    });

    it('dispatches the match payload when a match is assigned', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        const websocketId = server.connections.get('user1').websocketId;
        db.getConnectionStatuses.mockResolvedValue(
            new Map([
                [
                    'user1',
                    {
                        websocketId,
                        matchId: 1,
                        matchAuthToken: 'TOKEN',
                        host: 'h',
                        port: 'p',
                    },
                ],
            ])
        );

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);

        expect(ws.sent).toEqual([
            JSON.stringify({
                login: 'user1',
                host: 'h',
                port: 'p',
                matchAuthToken: 'TOKEN',
            }),
        ]);
        expect(ws.closed).toBe(true);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);
        expect(server.connections.has('user1')).toBe(false);
    });

    it('closes the connection whose websocket id changed and does not dispatch', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        db.getConnectionStatuses.mockResolvedValue(
            new Map([
                [
                    'user1',
                    {
                        websocketId: 'another-ws-id',
                        matchId: 1,
                        matchAuthToken: 'TOKEN',
                        host: 'h',
                        port: 'p',
                    },
                ],
            ])
        );

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);

        expect(ws.sent).toEqual([]);
        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4001);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);
        expect(server.connections.has('user1')).toBe(false);
    });

    it('closes the connection when the user no longer exists', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        db.getConnectionStatuses.mockResolvedValue(new Map());

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);

        expect(ws.sent).toEqual([]);
        expect(ws.closed).toBe(true);
        expect(ws.closeCode).toBe(4004);

        await vi.advanceTimersByTimeAsync(DEFAULT_OPTIONS.pollIntervalMs);
        expect(server.connections.has('user1')).toBe(false);
    });

    it('replaces an existing connection for the same login', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();

        const firstWs = new FakeWebSocket();
        await server.onConnection(firstWs, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });
        expect(server.connections.has('user1')).toBe(true);

        const secondWs = new FakeWebSocket();
        await server.onConnection(secondWs, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(firstWs.closed).toBe(true);
        expect(firstWs.closeCode).toBe(4001);
        expect(server.connections.has('user1')).toBe(true);
        expect(secondWs.closed).toBe(false);

        firstWs.emit('close', { code: 1000 });

        expect(server.connections.has('user1')).toBe(true);
        expect(secondWs.closed).toBe(false);
    });

    it('removes the connection and dequeues it when it closes', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });
        const connection = server.connections.get('user1');

        connection.close();

        expect(server.connections.has('user1')).toBe(false);
        expect(db.removeQueueStatus).toHaveBeenCalledWith(
            'user1',
            connection.websocketId
        );
    });

    it('does not remove a replacement connection when a stale one closes', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();

        const firstWs = new FakeWebSocket();
        await server.onConnection(firstWs, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });
        const firstConnection = server.connections.get('user1');

        const secondWs = new FakeWebSocket();
        await server.onConnection(secondWs, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });
        const secondConnection = server.connections.get('user1');

        db.removeQueueStatus.mockClear();
        server.closeConnection(firstConnection);

        expect(server.connections.get('user1')).toBe(secondConnection);
        expect(db.removeQueueStatus).not.toHaveBeenCalled();
    });

    it('extends the queue status for queued connections', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        await vi.advanceTimersByTimeAsync(
            DEFAULT_OPTIONS.queueExtensionIntervalMs
        );

        const connection = server.connections.get('user1');
        expect(db.extendQueueStatuses).toHaveBeenCalledWith([connection], 5000);
    });

    it('does not extend the queue status for matched connections', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });
        db.extendQueueStatuses.mockClear();
        await server.connections.get('user1').sendMatchAndClose({
            login: 'user1',
        });

        await vi.advanceTimersByTimeAsync(
            DEFAULT_OPTIONS.queueExtensionIntervalMs
        );

        expect(db.extendQueueStatuses).not.toHaveBeenCalled();
    });

    it('closes all connections on close', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        await server.close();

        expect(ws.closed).toBe(true);
        expect(server.connections.size).toBe(0);
    });

    it('does not run when the server is closed', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const server = await createServer();
        const ws = new FakeWebSocket();

        await server.close();

        await server.onConnection(ws, {
            headers: { cookie: `session=${VALID_TOKEN}` },
        });

        expect(server.state).toBe(State.CLOSED);
        expect(server.connections.has('user1')).toBe(false);
        expect(ws.closed).toBe(true);
        expect(db.setUserWebsocket).not.toHaveBeenCalled();

        await vi.advanceTimersByTimeAsync(
            DEFAULT_OPTIONS.queueExtensionIntervalMs +
                DEFAULT_OPTIONS.pollIntervalMs
        );

        expect(db.extendQueueStatuses).not.toHaveBeenCalled();
        expect(db.getConnectionStatuses).not.toHaveBeenCalled();
    });

    it('does not start a second time', async () => {
        const wss = {
            on: vi.fn(),
            close: vi.fn(),
        };
        const server = new ConnectionServer(wss);

        server.open();
        expect(server.state).toBe(State.OPEN);
        expect(wss.on).toHaveBeenCalledTimes(1);

        server.open();

        expect(server.state).toBe(State.OPEN);
        expect(wss.on).toHaveBeenCalledTimes(1);
    });

    it('does not close twice', async () => {
        const wss = {
            on: vi.fn(),
            close: vi.fn(),
        };
        const server = new ConnectionServer(wss);
        server.open();

        await server.close();
        await server.close();

        expect(server.state).toBe(State.CLOSED);
        expect(wss.close).toHaveBeenCalledTimes(1);
    });

    it('only handles upgrades on the connection path', () => {
        const handlers = {};
        const httpServer = {
            on: vi.fn((event, fn) => {
                handlers[event] = fn;
            }),
        };
        const wss = {
            on: vi.fn(),
            handleUpgrade: vi.fn(),
            emit: vi.fn(),
        };
        const server = new ConnectionServer(wss);
        server.attach(httpServer);

        expect(httpServer.on).toHaveBeenCalledWith(
            'upgrade',
            expect.any(Function)
        );

        const socket = {};
        const connectionRequest = { url: '/api/connection', headers: {} };
        handlers.upgrade(connectionRequest, socket, 'head');
        expect(wss.handleUpgrade).toHaveBeenCalledWith(
            connectionRequest,
            socket,
            'head',
            expect.any(Function)
        );

        const otherRequest = { url: '/api/other', headers: {} };
        const otherSocket = { destroyed: false, destroy() {} };
        handlers.upgrade(otherRequest, otherSocket, 'head');
        expect(wss.handleUpgrade).toHaveBeenCalledTimes(1);
    });

    it('destroys the socket when the request url is invalid', () => {
        const handlers = {};
        const httpServer = {
            on: vi.fn((event, fn) => {
                handlers[event] = fn;
            }),
        };
        const wss = {
            on: vi.fn(),
            handleUpgrade: vi.fn(),
            emit: vi.fn(),
        };
        const server = new ConnectionServer(wss);
        server.attach(httpServer);

        const socket = {
            destroyed: false,
            destroy() {
                this.destroyed = true;
            },
        };
        handlers.upgrade({ url: 'http://[', headers: {} }, socket, 'head');

        expect(socket.destroyed).toBe(true);
        expect(wss.handleUpgrade).not.toHaveBeenCalled();
    });

    it('detaches from the http server on close', async () => {
        const httpServer = {
            on: vi.fn(),
            off: vi.fn(),
        };
        const wss = {
            on: vi.fn(),
            handleUpgrade: vi.fn(),
            emit: vi.fn(),
            close: vi.fn(),
        };
        const server = new ConnectionServer(wss);
        server.attach(httpServer);

        await server.close();

        expect(server.httpServer).toBe(null);
        expect(server.upgradeHandler).toBe(null);
        expect(httpServer.off).toHaveBeenCalledWith(
            'upgrade',
            expect.any(Function)
        );
        expect(wss.close).toHaveBeenCalled();
    });

    it('does not attach twice', () => {
        const httpServer = {
            on: vi.fn(),
            off: vi.fn(),
        };
        const wss = {
            on: vi.fn(),
            handleUpgrade: vi.fn(),
            emit: vi.fn(),
        };
        const server = new ConnectionServer(wss);
        server.attach(httpServer);
        server.attach(httpServer);

        expect(httpServer.on).toHaveBeenCalledTimes(1);
    });
});
