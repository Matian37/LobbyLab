import {
    describe,
    it,
    expect,
    beforeEach,
    afterEach,
    vi,
    onTestFinished,
} from 'vitest';
import { EventEmitter } from 'node:events';
import {
    Connections,
    ConnectionServer,
    createConnection,
    parseSessionToken,
    createWebSocketServer,
} from '$lib/server.js';
import {
    State,
    SESSION_TOKEN_LENGTH,
    SERVER_DEFAULT_OPTIONS,
} from '$lib/constants.js';
import { ConnectionStateError } from '$lib/errors.js';
import * as db from '$lib/db.js';

const { Connection } = vi.hoisted(() => ({ Connection: vi.fn() }));

vi.mock('$lib/connection.js', () => ({ Connection }));

vi.mock('$lib/db.js', () => ({
    getLoginFromToken: vi.fn(),
    extendQueueStatuses: vi.fn(),
    removeQueueStatus: vi.fn(),
    setQueueStatus: vi.fn(),
    getConnectionStatuses: vi.fn(),
}));

const EXAMPLE_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);
const OPTIONS = Object.freeze({
    ...SERVER_DEFAULT_OPTIONS,
    queueExtensionIntervalMs: 3_000,
    queueExtensionMs: 5_000,
    pollIntervalMs: 2_500,
});

function createMockConnection({
    login = 'user1',
    websocketId = 1,
    state = State.OPEN,
} = {}) {
    const handlers = new Map();
    const connection = {
        login,
        websocketId,
        state,
        on: vi.fn((event, handler) => handlers.set(event, handler)),
        open: vi.fn(),
        close: vi.fn(),
        sendMatchAndClose: vi.fn(),
        emit(event, ...args) {
            handlers.get(event)?.(...args);
        },
    };
    return connection;
}

function mockWs() {
    return { close: vi.fn(), send: vi.fn(), ping: vi.fn() };
}

function mockWss() {
    return {
        on: vi.fn(),
        handleUpgrade: vi.fn(),
        emit: vi.fn(),
        close: vi.fn(),
    };
}

function makeServer({ open = false } = {}) {
    const httpServer = new EventEmitter();
    const wss = mockWss();
    const server = new ConnectionServer(httpServer, wss, OPTIONS);
    if (open) expect(server.open()).toBeUndefined();
    onTestFinished(() => server.close());
    return { server, httpServer, wss };
}

function makeValidRequest() {
    return { headers: { cookie: `session=${EXAMPLE_TOKEN}` } };
}

beforeEach(() => {
    vi.useFakeTimers({
        toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'],
    });
    vi.clearAllMocks();
    Connection.mockReset();
    Connection.mockImplementation(function (ws, login, websocketId, options) {
        return {
            ws,
            login,
            websocketId,
            options,
            state: State.OPEN,
            close: vi.fn(),
            open: vi.fn(),
            on: vi.fn(),
        };
    });
    db.getLoginFromToken.mockResolvedValue('user1');
    db.extendQueueStatuses.mockResolvedValue(undefined);
    db.removeQueueStatus.mockResolvedValue(undefined);
    db.setQueueStatus.mockResolvedValue(1);
    db.getConnectionStatuses.mockResolvedValue(new Map());
});

afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
});

describe('Connections.set', () => {
    it('stores the first connection and opens it', () => {
        const connections = new Connections();
        const connection = createMockConnection();

        expect(connections.set(connection)).toBeUndefined();

        expect(connection.open).toHaveBeenCalledTimes(1);
        expect(connection.on).toHaveBeenCalledWith(
            'close',
            expect.any(Function)
        );
        expect(connection.on.mock.invocationCallOrder[0]).toBeLessThan(
            connection.open.mock.invocationCallOrder[0]
        );
        expect(connections.getQueued()).toEqual([connection]);
    });

    it('replaces stale connection when websocketId is higher', () => {
        const connections = new Connections();
        const oldConnection = createMockConnection();
        const newConnection = createMockConnection({ websocketId: 2 });
        expect(connections.set(oldConnection)).toBeUndefined();
        const deleteSpy = vi.spyOn(connections, 'delete');

        expect(connections.set(newConnection)).toBeUndefined();

        expect(oldConnection.close).toHaveBeenCalledWith(
            4001,
            'Replaced by new connection'
        );
        expect(newConnection.on).toHaveBeenCalledWith(
            'close',
            expect.any(Function)
        );
        expect(newConnection.open).toHaveBeenCalledTimes(1);
        expect(connections.getQueued()).toEqual([newConnection]);

        oldConnection.emit('close');

        expect(deleteSpy).toHaveBeenCalledWith(
            oldConnection.login,
            oldConnection.websocketId
        );
    });

    it('closes newcomer when websocketId is lower', () => {
        const connections = new Connections();
        const existingConnection = createMockConnection({ websocketId: 2 });
        const newcomerConnection = createMockConnection();
        expect(connections.set(existingConnection)).toBeUndefined();

        expect(connections.set(newcomerConnection)).toBeUndefined();

        expect(newcomerConnection.close).toHaveBeenCalledWith(
            4001,
            'Replaced by new connection'
        );
        expect(existingConnection.close).not.toHaveBeenCalled();
        expect(newcomerConnection.open).not.toHaveBeenCalled();
        expect(newcomerConnection.on).not.toHaveBeenCalled();
        expect(connections.getQueued()).toEqual([existingConnection]);
    });
});

describe('Connections.delete', () => {
    it('removes connection and closes it', () => {
        const connections = new Connections();
        const connection = createMockConnection();
        expect(connections.set(connection)).toBeUndefined();

        db.removeQueueStatus.mockResolvedValue(undefined);
        expect(
            connections.delete(connection.login, connection.websocketId)
        ).toBeUndefined();

        expect(connection.close).toHaveBeenCalledTimes(1);
        expect(db.removeQueueStatus).toHaveBeenCalledWith(
            connection.login,
            connection.websocketId
        );
        expect(connections.getQueued()).toEqual([]);
    });

    it('does nothing when login is missing', () => {
        const connections = new Connections();
        const connection = createMockConnection();
        expect(connections.set(connection)).toBeUndefined();

        expect(
            connections.delete('other-user', connection.websocketId)
        ).toBeUndefined();

        expect(connection.close).not.toHaveBeenCalled();
        expect(db.removeQueueStatus).not.toHaveBeenCalled();
        expect(connections.getQueued()).toEqual([connection]);
    });

    it('keeps connection when websocketId mismatches', () => {
        const connections = new Connections();
        const connection = createMockConnection();
        expect(connections.set(connection)).toBeUndefined();

        expect(connections.delete(connection.login, 99)).toBeUndefined();

        expect(connection.close).not.toHaveBeenCalled();
        expect(db.removeQueueStatus).not.toHaveBeenCalled();
        expect(connections.getQueued()).toEqual([connection]);
    });

    it('removes connection when removeQueueStatus rejects', () => {
        const connections = new Connections();
        const connection = createMockConnection();
        expect(connections.set(connection)).toBeUndefined();
        db.removeQueueStatus.mockRejectedValue(new Error('database is down'));

        expect(
            connections.delete(connection.login, connection.websocketId)
        ).toBeUndefined();

        expect(db.removeQueueStatus).toHaveBeenCalledWith(
            connection.login,
            connection.websocketId
        );
    });
});

describe('Connections.getQueued', () => {
    it('returns only connections in open state', () => {
        const connections = new Connections();
        const openConnection = createMockConnection({
            login: 'open',
            state: State.OPEN,
        });
        const initConnection = createMockConnection({
            login: 'init',
            websocketId: 2,
            state: State.INIT,
        });
        const closedConnection = createMockConnection({
            login: 'closed',
            websocketId: 3,
            state: State.CLOSED,
        });
        expect(connections.set(openConnection)).toBeUndefined();
        expect(connections.set(initConnection)).toBeUndefined();
        expect(connections.set(closedConnection)).toBeUndefined();

        expect(connections.getQueued()).toEqual([openConnection]);
    });

    it('returns empty array without connections', () => {
        const connections = new Connections();

        expect(connections.getQueued()).toEqual([]);
    });
});

describe('Connections.close', () => {
    it('closes every connection in the map', () => {
        const connections = new Connections();
        const first = createMockConnection();
        const second = createMockConnection({ login: 'user2', websocketId: 2 });
        expect(connections.set(first)).toBeUndefined();
        expect(connections.set(second)).toBeUndefined();

        expect(connections.close()).toBeUndefined();

        expect(first.close).toHaveBeenCalledTimes(1);
        expect(second.close).toHaveBeenCalledTimes(1);
        expect(connections.getQueued()).toEqual([]);
    });

    it('handles an empty map', () => {
        const connections = new Connections();
        expect(connections.close()).toBeUndefined();
    });
});

describe('createConnection', () => {
    it('returns a connection with the db websocketId', async () => {
        const ws = mockWs();
        db.setQueueStatus.mockResolvedValue(5);

        const connection = await createConnection(ws, 'user1', OPTIONS);

        expect(db.setQueueStatus).toHaveBeenCalledWith(
            'user1',
            OPTIONS.queueExtensionMs
        );
        expect(Connection).toHaveBeenCalledWith(ws, 'user1', 5, OPTIONS);
        expect(connection).toEqual(
            expect.objectContaining({ ws, login: 'user1', websocketId: 5 })
        );
    });

    it('returns null and closes ws when db fails', async () => {
        const ws = mockWs();
        db.setQueueStatus.mockRejectedValue(new Error('database is down'));

        const result = await createConnection(ws, 'user1', OPTIONS);

        expect(result).toBeNull();
        expect(ws.close).toHaveBeenCalledWith(1011, 'Internal error');
        expect(Connection).not.toHaveBeenCalled();
    });

    it('returns null and closes ws when already in match', async () => {
        const ws = mockWs();
        db.setQueueStatus.mockResolvedValue(null);

        const result = await createConnection(ws, 'user1', OPTIONS);

        expect(result).toBeNull();
        expect(ws.close).toHaveBeenCalledWith(4000, 'Already in match');
        expect(Connection).not.toHaveBeenCalled();
    });
});

describe('parseSessionToken', () => {
    it.each([
        [
            'valid session cookie',
            { headers: { cookie: `session=${EXAMPLE_TOKEN}` } },
            { data: EXAMPLE_TOKEN },
        ],
        ['missing cookie', { headers: {} }, { error: 'No cookie', code: 4401 }],
        [
            'oversized cookie',
            { headers: { cookie: 'x'.repeat(8193) } },
            { error: 'Cookie header too large', code: 4401 },
        ],
        [
            'missing session token',
            { headers: { cookie: 'other=value' } },
            { error: 'No session token', code: 4401 },
        ],
        [
            'invalid session token',
            { headers: { cookie: 'session=invalid' } },
            { error: 'Invalid session token', code: 4401 },
        ],
    ])('handles %s', (_name, request, expected) => {
        expect(parseSessionToken(request)).toEqual(expected);
    });
});

describe('ConnectionServer', () => {
    it('starts in INIT without timers or listeners', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);

        expect(server.state).toBe(State.INIT);
        expect(httpServer.listenerCount('upgrade')).toBe(0);
        expect(wss.on).not.toHaveBeenCalled();
    });
});

describe('ConnectionServer.open', () => {
    it('opens and starts timers and registers handlers', async () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        const extendQueuesSpy = vi.spyOn(server, 'extendQueues');
        const onPullSpy = vi.spyOn(server, 'onPull');
        const onConnectionSpy = vi.spyOn(server, 'onConnection');
        onTestFinished(() => server.close());

        expect(server.open()).toBeUndefined();

        expect(server.state).toBe(State.OPEN);
        expect(httpServer.listenerCount('upgrade')).toBe(1);

        await vi.advanceTimersByTimeAsync(OPTIONS.queueExtensionIntervalMs);
        expect(extendQueuesSpy).toHaveBeenCalled();

        await vi.advanceTimersByTimeAsync(OPTIONS.pollIntervalMs);
        expect(onPullSpy).toHaveBeenCalled();

        const [, connectionHandler] = wss.on.mock.calls.find(
            ([event]) => event === 'connection'
        );
        const ws = mockWs();
        const request = makeValidRequest();
        connectionHandler(ws, request);
        expect(onConnectionSpy).toHaveBeenCalledWith(ws, request);
    });

    it('throws when opened twice', () => {
        const { server } = makeServer({ open: true });

        expect(() => server.open()).toThrow(ConnectionStateError);
    });
});

describe('ConnectionServer.upgradeHandler', () => {
    it('upgrades when pathname matches', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        expect(server.open()).toBeUndefined();
        onTestFinished(() => server.close());
        const request = { url: OPTIONS.connectionPath };
        const socket = { destroy: vi.fn() };
        const head = Buffer.alloc(0);

        httpServer.emit('upgrade', request, socket, head);

        expect(wss.handleUpgrade).toHaveBeenCalledWith(
            request,
            socket,
            head,
            expect.any(Function)
        );
        expect(socket.destroy).not.toHaveBeenCalled();

        // wss is mocked, so simulate handleUpgrade completing its handshake
        // and invoking its callback with the upgraded websocket
        const callback = wss.handleUpgrade.mock.calls[0][3];
        const upgradedWs = {};
        callback(upgradedWs);

        expect(wss.emit).toHaveBeenCalledWith(
            'connection',
            upgradedWs,
            request
        );
    });

    it('destroys socket when pathname does not match', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        expect(server.open()).toBeUndefined();
        onTestFinished(() => server.close());
        const socket = { destroy: vi.fn() };

        httpServer.emit('upgrade', { url: '/other' }, socket, Buffer.alloc(0));

        expect(socket.destroy).toHaveBeenCalledTimes(1);
        expect(wss.handleUpgrade).not.toHaveBeenCalled();
    });

    it('destroys socket when url is invalid', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        expect(server.open()).toBeUndefined();
        onTestFinished(() => server.close());
        const socket = { destroy: vi.fn() };

        httpServer.emit(
            'upgrade',
            { url: 'http://[invalid' },
            socket,
            Buffer.alloc(0)
        );

        expect(socket.destroy).toHaveBeenCalledTimes(1);
        expect(wss.handleUpgrade).not.toHaveBeenCalled();
    });

    it('skips upgrade when server is closed', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        expect(server.open()).toBeUndefined();
        expect(server.close()).toBeUndefined();
        const socket = { destroy: vi.fn() };

        httpServer.emit(
            'upgrade',
            { url: OPTIONS.connectionPath },
            socket,
            Buffer.alloc(0)
        );

        expect(wss.handleUpgrade).not.toHaveBeenCalled();
        expect(socket.destroy).not.toHaveBeenCalled();
    });
});

describe('ConnectionServer.extendQueues', () => {
    it('does nothing when server is not open', async () => {
        const server = new ConnectionServer(
            new EventEmitter(),
            mockWss(),
            OPTIONS
        );

        expect(await server.extendQueues()).toBeUndefined();

        expect(db.extendQueueStatuses).not.toHaveBeenCalled();
    });

    it('extends queue statuses for queued connections', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        const closedConnection = createMockConnection({
            login: 'closed',
            state: State.CLOSED,
        });
        expect(server.connections.set(connection)).toBeUndefined();
        expect(server.connections.set(closedConnection)).toBeUndefined();

        expect(await server.extendQueues()).toBeUndefined();

        expect(db.extendQueueStatuses).toHaveBeenCalledWith(
            [connection],
            OPTIONS.queueExtensionMs
        );
    });

    it('does not throw when extendQueueStatuses fails', async () => {
        const { server } = makeServer({ open: true });
        db.extendQueueStatuses.mockRejectedValue(new Error('database is down'));

        expect(await server.extendQueues()).toBeUndefined();
    });
});

describe('ConnectionServer.onPull', () => {
    it('does nothing when server is not open', async () => {
        const server = new ConnectionServer(
            new EventEmitter(),
            mockWss(),
            OPTIONS
        );

        expect(await server.onPull()).toBeUndefined();

        expect(db.getConnectionStatuses).not.toHaveBeenCalled();
    });

    it('does not throw when getConnectionStatuses fails', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        expect(server.connections.set(connection)).toBeUndefined();
        db.getConnectionStatuses.mockRejectedValue(
            new Error('database is down')
        );

        expect(await server.onPull()).toBeUndefined();

        expect(connection.close).not.toHaveBeenCalled();
        expect(connection.sendMatchAndClose).not.toHaveBeenCalled();
    });

    it('closes connection missing from statuses', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        expect(server.connections.set(connection)).toBeUndefined();

        expect(await server.onPull()).toBeUndefined();

        expect(connection.close).toHaveBeenCalledWith(
            4004,
            'User no longer exists'
        );
    });

    it('closes connection with stale websocketId', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        expect(server.connections.set(connection)).toBeUndefined();
        db.getConnectionStatuses.mockResolvedValue(
            new Map([[connection.login, { websocketId: 99, matchId: null }]])
        );

        expect(await server.onPull()).toBeUndefined();

        expect(connection.close).toHaveBeenCalledWith(
            4001,
            'Replaced by new connection'
        );
    });

    it('sends match payload when match is ready', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        expect(server.connections.set(connection)).toBeUndefined();
        db.getConnectionStatuses.mockResolvedValue(
            new Map([
                [
                    connection.login,
                    {
                        websocketId: connection.websocketId,
                        matchId: 7,
                        host: 'host',
                        port: 'port',
                        matchAuthToken: 'EXAMPLE_TOKEN',
                    },
                ],
            ])
        );

        expect(await server.onPull()).toBeUndefined();

        expect(connection.sendMatchAndClose).toHaveBeenCalledWith({
            login: connection.login,
            host: 'host',
            port: 'port',
            matchAuthToken: 'EXAMPLE_TOKEN',
        });
        expect(connection.close).not.toHaveBeenCalled();
    });

    it('skips connection when matchId is null', async () => {
        const { server } = makeServer({ open: true });
        const connection = createMockConnection();
        expect(server.connections.set(connection)).toBeUndefined();
        db.getConnectionStatuses.mockResolvedValue(
            new Map([
                [
                    connection.login,
                    { websocketId: connection.websocketId, matchId: null },
                ],
            ])
        );

        expect(await server.onPull()).toBeUndefined();

        expect(connection.sendMatchAndClose).not.toHaveBeenCalled();
        expect(connection.close).not.toHaveBeenCalled();
    });

    it('processes multiple connections', async () => {
        const { server } = makeServer({ open: true });
        const first = createMockConnection();
        const second = createMockConnection({ login: 'user2', websocketId: 2 });
        expect(server.connections.set(first)).toBeUndefined();
        expect(server.connections.set(second)).toBeUndefined();
        db.getConnectionStatuses.mockResolvedValue(
            new Map([
                [
                    first.login,
                    { websocketId: first.websocketId, matchId: null },
                ],
                [
                    second.login,
                    { websocketId: second.websocketId, matchId: null },
                ],
            ])
        );

        expect(await server.onPull()).toBeUndefined();

        expect(first.sendMatchAndClose).not.toHaveBeenCalled();
        expect(first.close).not.toHaveBeenCalled();
        expect(second.sendMatchAndClose).not.toHaveBeenCalled();
        expect(second.close).not.toHaveBeenCalled();
    });
});

describe('ConnectionServer.onConnection', () => {
    it('closes ws when server is closed', async () => {
        const server = new ConnectionServer(
            new EventEmitter(),
            mockWss(),
            OPTIONS
        );
        expect(server.open()).toBeUndefined();
        expect(server.close()).toBeUndefined();
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');

        expect(
            await server.onConnection(ws, makeValidRequest())
        ).toBeUndefined();

        expect(ws.close).toHaveBeenCalledWith(1001, 'Server is shutting down');
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
        expect(db.setQueueStatus).not.toHaveBeenCalled();
        expect(connectionsSetSpy).not.toHaveBeenCalled();
    });

    it('closes ws when cookie parsing failed', async () => {
        const { server } = makeServer({ open: true });
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');

        expect(await server.onConnection(ws, { headers: {} })).toBeUndefined();

        expect(ws.close).toHaveBeenCalledWith(4401, 'No cookie');
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
        expect(db.setQueueStatus).not.toHaveBeenCalled();
        expect(connectionsSetSpy).not.toHaveBeenCalled();
    });

    it('closes ws when getLoginFromToken throws', async () => {
        const { server } = makeServer({ open: true });
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');
        db.getLoginFromToken.mockRejectedValue(new Error('database is down'));

        expect(
            await server.onConnection(ws, makeValidRequest())
        ).toBeUndefined();

        expect(ws.close).toHaveBeenCalledWith(1011, 'Internal error');
        expect(db.setQueueStatus).not.toHaveBeenCalled();
        expect(connectionsSetSpy).not.toHaveBeenCalled();
    });

    it('closes ws when login is not found', async () => {
        const { server } = makeServer({ open: true });
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');
        db.getLoginFromToken.mockResolvedValue(null);

        expect(
            await server.onConnection(ws, makeValidRequest())
        ).toBeUndefined();

        expect(ws.close).toHaveBeenCalledWith(4401, 'Invalid session token');
        expect(db.setQueueStatus).not.toHaveBeenCalled();
        expect(connectionsSetSpy).not.toHaveBeenCalled();
    });

    it('closes ws when server closed during creation', async () => {
        const { server } = makeServer({ open: true });
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');
        let resolveSetQueue;
        db.setQueueStatus.mockImplementation(
            () => new Promise((resolve) => (resolveSetQueue = resolve))
        );

        const pending = server.onConnection(ws, makeValidRequest());
        await Promise.resolve();
        expect(db.setQueueStatus).toHaveBeenCalled();

        expect(server.close()).toBeUndefined();
        resolveSetQueue(1);
        await pending;

        expect(ws.close).toHaveBeenCalledWith(1001, 'Server is shutting down');
        expect(connectionsSetSpy).not.toHaveBeenCalled();
    });

    it('registers the created connection', async () => {
        const { server } = makeServer({ open: true });
        const ws = mockWs();
        const connectionsSetSpy = vi.spyOn(server.connections, 'set');

        expect(
            await server.onConnection(ws, makeValidRequest())
        ).toBeUndefined();

        expect(db.getLoginFromToken).toHaveBeenCalledWith(EXAMPLE_TOKEN);
        expect(db.setQueueStatus).toHaveBeenCalledWith(
            'user1',
            OPTIONS.queueExtensionMs
        );
        expect(connectionsSetSpy).toHaveBeenCalledWith(
            expect.objectContaining({ login: 'user1', websocketId: 1 })
        );
    });
});

describe('ConnectionServer.close', () => {
    it('closes cleanly and detaches handlers', async () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        const offSpy = vi.spyOn(httpServer, 'off');
        const connectionsCloseSpy = vi.spyOn(server.connections, 'close');
        const extendQueuesSpy = vi.spyOn(server, 'extendQueues');
        const onPullSpy = vi.spyOn(server, 'onPull');
        expect(server.open()).toBeUndefined();

        expect(server.close()).toBeUndefined();

        expect(server.state).toBe(State.CLOSED);
        await vi.advanceTimersByTimeAsync(OPTIONS.pollIntervalMs * 2);
        expect(extendQueuesSpy).not.toHaveBeenCalled();
        expect(onPullSpy).not.toHaveBeenCalled();

        expect(offSpy).toHaveBeenCalledWith('upgrade', expect.any(Function));
        expect(connectionsCloseSpy).toHaveBeenCalledTimes(1);
        expect(wss.close).toHaveBeenCalledTimes(1);

        // closes happen in order: httpServer.off first, then connections, then wss
        expect(offSpy.mock.invocationCallOrder[0]).toBeLessThan(
            connectionsCloseSpy.mock.invocationCallOrder[0]
        );
        expect(connectionsCloseSpy.mock.invocationCallOrder[0]).toBeLessThan(
            wss.close.mock.invocationCallOrder[0]
        );
    });

    it('is idempotent when closed twice', () => {
        const httpServer = new EventEmitter();
        const wss = mockWss();
        const server = new ConnectionServer(httpServer, wss, OPTIONS);
        const offSpy = vi.spyOn(httpServer, 'off');
        const connectionsCloseSpy = vi.spyOn(server.connections, 'close');
        expect(server.open()).toBeUndefined();

        expect(server.close()).toBeUndefined();
        expect(server.close()).toBeUndefined();

        expect(offSpy).toHaveBeenCalledTimes(1);
        expect(connectionsCloseSpy).toHaveBeenCalledTimes(1);
        expect(wss.close).toHaveBeenCalledTimes(1);
    });
});

describe('createWebSocketServer', () => {
    it('creates and opens a connection server', async () => {
        const httpServer = new EventEmitter();
        const server = await createWebSocketServer(httpServer, OPTIONS);
        onTestFinished(() => server.close());

        expect(server).toBeInstanceOf(ConnectionServer);
        expect(server.httpServer).toBe(httpServer);
        expect(server.options).toBe(OPTIONS);
        expect(server.state).toBe(State.OPEN);
    });
});
