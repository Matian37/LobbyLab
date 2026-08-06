import {
    describe,
    it,
    expect,
    beforeEach,
    afterEach,
    vi,
    onTestFinished,
} from 'vitest';
import { once } from 'node:events';
import { WebSocket, WebSocketServer } from 'ws';
import { Connection } from '$lib/server/connection.js';
import { CONNECTION_DEFAULT_OPTIONS, State } from '$lib/constants.js';
import { ConnectionStateError } from '$lib/errors.js';

const OPTIONS = Object.freeze({
    ...CONNECTION_DEFAULT_OPTIONS,
    pingIntervalMs: 10_000,
    pongTimeoutMs: 40,
    maxMissedPongs: 2,
});

function makeConnection(serverSocket, options = OPTIONS) {
    const connection = new Connection(serverSocket, 'user1', 1, options);
    onTestFinished(() => connection.close());
    return connection;
}

async function createSocket({ autoPong = true } = {}) {
    const wss = new WebSocketServer({ port: 0, host: '127.0.0.1' });
    await new Promise((resolve, reject) => {
        wss.once('listening', resolve);
        wss.once('error', reject);
    });

    const { port } = wss.address();
    const client = new WebSocket(`ws://127.0.0.1:${port}`, { autoPong });
    client.on('error', () => {});

    const controller = new AbortController();
    const { signal } = controller;

    try {
        const [connectionArgs] = await Promise.all([
            once(wss, 'connection', { signal }),
            once(client, 'open', { signal }),
        ]);

        const [serverSocket] = connectionArgs;
        serverSocket.on('error', () => {});

        onTestFinished(async () => {
            client.terminate();
            serverSocket.terminate();
            await new Promise((resolve) => wss.close(resolve));
        });

        return { serverSocket, client, wss };
    } catch (err) {
        controller.abort();
        client.terminate();
        wss.close();
        throw err;
    }
}

beforeEach(() => {
    vi.useFakeTimers({
        toFake: ['setTimeout', 'clearTimeout', 'setInterval', 'clearInterval'],
    });
});

afterEach(() => {
    vi.restoreAllMocks();
    vi.useRealTimers();
});

describe('open', () => {
    it('opens the connection', async () => {
        const { serverSocket } = await createSocket();
        const connection = makeConnection(serverSocket);

        const counts = {
            pong: serverSocket.listenerCount('pong'),
            close: serverSocket.listenerCount('close'),
            error: serverSocket.listenerCount('error'),
        };

        expect(connection.open()).toBeUndefined();

        expect(connection.state).toBe(State.OPEN);
        expect(connection.pingTimer).not.toBe(null);
        expect(connection.waitingForPing).toBe(false);
        expect(serverSocket.listenerCount('pong')).toBe(counts.pong + 1);
        expect(serverSocket.listenerCount('close')).toBe(counts.close + 1);
        expect(serverSocket.listenerCount('error')).toBe(counts.error + 1);
    });

    it('throws when opened twice', async () => {
        const { serverSocket } = await createSocket();
        const connection = makeConnection(serverSocket);

        expect(connection.open()).toBeUndefined();

        expect(() => connection.open()).toThrow(ConnectionStateError);
        expect(connection.state).toBe(State.OPEN);
    });
});

describe('close', () => {
    it('closes gracefully', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const counts = {
            pong: serverSocket.listenerCount('pong'),
            close: serverSocket.listenerCount('close'),
            error: serverSocket.listenerCount('error'),
        };
        const onClose = vi.fn();
        connection.on('close', onClose);

        expect(connection.open()).toBeUndefined();
        expect(serverSocket.listenerCount('pong')).toBe(counts.pong + 1);

        expect(connection.close(4001, 'Replaced')).toBeUndefined();

        expect(connection.state).toBe(State.CLOSED);
        expect(connection.pingTimer).toBe(null);
        expect(connection.pongTimer).toBe(null);
        expect(serverSocket.listenerCount('pong')).toBe(counts.pong);
        expect(serverSocket.listenerCount('close')).toBe(counts.close);
        expect(serverSocket.listenerCount('error')).toBe(counts.error);
        expect(onClose).toHaveBeenCalledTimes(1);

        const [code, reason] = await once(client, 'close');
        expect(code).toBe(4001);
        expect(reason.toString()).toBe('Replaced');
    });

    it('terminates when hard closed', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);

        expect(connection.open()).toBeUndefined();

        for (let i = 0; i < connection.options.maxMissedPongs + 1; i++) {
            expect(connection.sendPing()).toBeUndefined();
            await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);
        }

        expect(connection.state).toBe(State.CLOSED);
        const [code] = await once(client, 'close');
        expect(code).toBe(1006);
    });

    it('ignores a second close', async () => {
        const { serverSocket } = await createSocket();
        const connection = makeConnection(serverSocket);
        const onClose = vi.fn();
        connection.on('close', onClose);

        expect(connection.open()).toBeUndefined();
        expect(connection.close()).toBeUndefined();
        expect(connection.close()).toBeUndefined();

        expect(connection.state).toBe(State.CLOSED);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('closes without being opened', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const onClose = vi.fn();
        connection.on('close', onClose);

        expect(connection.close()).toBeUndefined();

        expect(connection.state).toBe(State.CLOSED);
        expect(connection.pingTimer).toBe(null);
        expect(connection.pongTimer).toBe(null);
        expect(onClose).toHaveBeenCalledTimes(1);

        const [code] = await once(client, 'close');
        expect(code).toBe(1000);
    });
});

describe('ping', () => {
    it('pings on the ping interval', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket, {
            ...OPTIONS,
            pingIntervalMs: 100,
        });
        const pings = [];
        client.on('ping', (data) => pings.push(String(data)));

        expect(connection.open()).toBeUndefined();

        const pongReceived = once(serverSocket, 'pong');
        const firstPing = once(client, 'ping');

        await vi.advanceTimersByTimeAsync(connection.options.pingIntervalMs);
        await firstPing;
        expect(pings).toEqual(['0']);
        expect(connection.waitingForPing).toBe(true);

        await pongReceived;

        const secondPing = once(client, 'ping');
        await vi.advanceTimersByTimeAsync(connection.options.pingIntervalMs);
        await secondPing;
        expect(pings).toEqual(['0', '1']);
        expect(connection.waitingForPing).toBe(true);
    });

    it('closes the connection when sending a ping fails', async () => {
        const { serverSocket } = await createSocket();
        const connection = makeConnection(serverSocket);
        vi.spyOn(serverSocket, 'ping').mockImplementation(() => {
            throw new Error('ping failed');
        });

        expect(connection.open()).toBeUndefined();
        expect(connection.sendPing()).toBeUndefined();

        expect(connection.state).toBe(State.CLOSED);
    });
});

describe('onPongMiss', () => {
    it('counts a missed pong', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);
        const pings = [];
        client.on('ping', (data) => pings.push(String(data)));

        expect(connection.open()).toBeUndefined();

        expect(connection.sendPing()).toBeUndefined();
        await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);

        expect(connection.missedPongs).toBe(1);
        expect(connection.pongTimer).toBe(null);

        const pingReceived = once(client, 'ping');
        expect(connection.sendPing()).toBeUndefined();
        await pingReceived;
        expect(pings).toEqual(['0', '1']);
    });

    it('closes after exceeding the missed pong limit', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);

        expect(connection.open()).toBeUndefined();

        for (let i = 0; i < connection.options.maxMissedPongs; i++) {
            expect(connection.sendPing()).toBeUndefined();
            await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);
        }

        expect(connection.missedPongs).toBe(connection.options.maxMissedPongs);
        expect(connection.state).toBe(State.OPEN);

        expect(connection.sendPing()).toBeUndefined();
        await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);

        expect(connection.missedPongs).toBe(
            connection.options.maxMissedPongs + 1
        );
        expect(connection.state).toBe(State.CLOSED);

        const [code] = await once(client, 'close');
        expect(code).toBe(1006);
    });

    it('ignores a missed pong when not waiting for one', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);
        const pings = [];
        client.on('ping', (data) => pings.push(String(data)));

        expect(connection.open()).toBeUndefined();

        expect(connection.sendPing()).toBeUndefined();
        expect(connection.waitingForPing).toBe(true);
        await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);
        expect(connection.missedPongs).toBe(1);
        expect(connection.waitingForPing).toBe(false);

        expect(connection.onPongMiss(1)).toBeUndefined();

        expect(connection.missedPongs).toBe(1);
        expect(connection.state).toBe(State.OPEN);
        expect(connection.pongTimer).toBe(null);
        expect(connection.waitingForPing).toBe(false);

        const pingReceived = once(client, 'ping');
        expect(connection.sendPing()).toBeUndefined();
        await pingReceived;
        expect(pings).toEqual(['0', '1']);
    });

    it('ignores a missed pong after close', async () => {
        const { serverSocket } = await createSocket({ autoPong: false });
        const connection = makeConnection(serverSocket);

        expect(connection.open()).toBeUndefined();
        expect(connection.close()).toBeUndefined();

        expect(connection.onPongMiss(0)).toBeUndefined();
        expect(connection.state).toBe(State.CLOSED);
        expect(connection.missedPongs).toBe(0);
    });
});

describe('onPong', () => {
    it('resets the missed pong counter on a pong', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);
        const pings = [];
        client.on('ping', (data) => pings.push(String(data)));

        expect(connection.open()).toBeUndefined();

        expect(connection.sendPing()).toBeUndefined();
        await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);
        expect(connection.sendPing()).toBeUndefined();
        await vi.advanceTimersByTimeAsync(connection.options.pongTimeoutMs);
        expect(connection.missedPongs).toBe(2);
        expect(connection.state).toBe(State.OPEN);

        const pongReceived = once(serverSocket, 'pong');
        expect(connection.sendPing()).toBeUndefined();
        expect(connection.waitingForPing).toBe(true);
        client.pong('2');
        await pongReceived;

        expect(connection.waitingForPing).toBe(false);
        expect(connection.missedPongs).toBe(0);
        expect(connection.pongTimer).toBe(null);
        expect(pings).toEqual(['0', '1', '2']);
    });

    it('ignores a pong after close', async () => {
        const { serverSocket } = await createSocket();
        const connection = makeConnection(serverSocket);

        expect(connection.open()).toBeUndefined();
        expect(connection.close()).toBeUndefined();

        expect(connection.onPong('0')).toBeUndefined();
        expect(connection.state).toBe(State.CLOSED);
        expect(connection.missedPongs).toBe(0);
    });

    it('ignores a pong when no ping is waiting for one', async () => {
        const { serverSocket, client } = await createSocket({
            autoPong: false,
        });
        const connection = makeConnection(serverSocket);
        const pings = [];
        client.on('ping', (data) => pings.push(String(data)));

        expect(connection.open()).toBeUndefined();

        const firstPong = once(serverSocket, 'pong');
        expect(connection.sendPing()).toBeUndefined();
        client.pong('0');
        await firstPong;
        expect(connection.missedPongs).toBe(0);

        const extraPong = once(serverSocket, 'pong');
        client.pong('1');
        await extraPong;

        const pingReceived = once(client, 'ping');
        expect(connection.sendPing()).toBeUndefined();
        await pingReceived;
        expect(connection.missedPongs).toBe(0);
        expect(pings).toEqual(['0', '1']);
    });
});

describe('sendMatchAndClose', () => {
    it('sends the payload and closes', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const received = once(client, 'message').then(([data]) =>
            JSON.parse(data.toString())
        );

        expect(connection.open()).toBeUndefined();

        const payload = {
            login: 'user1',
            host: 'h',
            port: 'p',
            matchAuthToken: 'TOKEN',
        };
        expect(connection.sendMatchAndClose(payload)).toBeUndefined();

        await expect(received).resolves.toEqual(payload);
        expect(connection.state).toBe(State.CLOSED);

        const [code] = await once(client, 'close');
        expect(code).toBe(1000);
    });

    it('closes the connection when sending the payload fails', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const messages = [];
        client.on('message', (data) => messages.push(data.toString()));
        vi.spyOn(serverSocket, 'send').mockImplementation(() => {
            throw new Error('send failed');
        });

        expect(connection.open()).toBeUndefined();
        expect(
            connection.sendMatchAndClose({ login: 'user1', host: 'h' })
        ).toBeUndefined();

        expect(messages).toEqual([]);
        expect(connection.state).toBe(State.CLOSED);
    });

    it('closes instead of sending when the socket is dead', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const messages = [];
        client.on('message', (data) => messages.push(data.toString()));
        const onClose = vi.fn();
        connection.on('close', onClose);

        expect(connection.open()).toBeUndefined();
        expect(connection.state).toBe(State.OPEN);

        vi.spyOn(serverSocket, 'readyState', 'get').mockReturnValue(
            WebSocket.CLOSED
        );
        expect(
            connection.sendMatchAndClose({ login: 'user1' })
        ).toBeUndefined();

        expect(messages).toEqual([]);
        expect(connection.state).toBe(State.CLOSED);
        expect(onClose).toHaveBeenCalledTimes(1);
    });

    it('does nothing when already closed', async () => {
        const { serverSocket, client } = await createSocket();
        const connection = makeConnection(serverSocket);
        const messages = [];
        client.on('message', (data) => messages.push(data.toString()));

        expect(connection.open()).toBeUndefined();
        expect(connection.close()).toBeUndefined();
        expect(
            connection.sendMatchAndClose({ login: 'user1', host: 'h' })
        ).toBeUndefined();

        expect(messages).toEqual([]);
        expect(connection.state).toBe(State.CLOSED);
    });
});
