import { EventEmitter } from 'node:events';
import { randomUUID } from 'node:crypto';
import { Mutex } from 'async-mutex';
import { WebSocket, WebSocketServer } from 'ws';
import {
    getLoginFromToken,
    extendQueueStatuses,
    removeQueueStatus,
    setUserWebsocket,
    getConnectionStatuses,
} from './db.js';

export const DEFAULT_OPTIONS = {
    connectionPath: '/api/connection',
    pingIntervalMs: 5_000,
    pongTimeoutMs: 3_000,
    maxMissedPongs: 2,
    queueExtensionIntervalMs: 3_000,
    queueExtensionMs: 5_000,
    pollIntervalMs: 2_500,
};

const HARD_CLOSE = Symbol('hard close');

export const State = Object.freeze({
    INIT: 'INIT',
    OPEN: 'OPEN',
    CLOSED: 'CLOSED',
});

export class Connection extends EventEmitter {
    #ws;
    #websocketId;
    #login;
    #state = State.INIT;
    #missedPongs = 0;
    #pingTimer = null;
    #pongTimer = null;
    #pingSeq = 0;
    #onPongHandler = null;
    #onCloseHandler = null;
    #onErrorHandler = null;

    constructor(ws, login, websocketId = randomUUID()) {
        super();
        this.#ws = ws;
        this.#websocketId = websocketId;
        this.#login = login;
    }

    get websocketId() {
        return this.#websocketId;
    }

    get login() {
        return this.#login;
    }

    get state() {
        return this.#state;
    }

    get missedPongs() {
        return this.#missedPongs;
    }

    get pingTimer() {
        return this.#pingTimer;
    }

    open() {
        if (this.#state !== State.INIT) return;
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.close();
            return;
        }

        this.#state = State.OPEN;

        this.#onPongHandler = (data) => this.onPong(data);
        this.#onCloseHandler = () => this.close();
        this.#onErrorHandler = () => this.close();

        this.#ws.on('pong', this.#onPongHandler);
        this.#ws.on('close', this.#onCloseHandler);
        this.#ws.on('error', this.#onErrorHandler);

        this.#pingTimer = setInterval(
            () => this.sendPing(),
            DEFAULT_OPTIONS.pingIntervalMs
        );
    }

    sendPing() {
        if (this.#state !== State.OPEN) return;
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.close();
            return;
        }

        clearTimeout(this.#pongTimer);

        const seq = ++this.#pingSeq;
        try {
            this.#ws.ping(String(seq));
        } catch (err) {
            console.debug('failed to send ping:', err);
            this.close();
            return;
        }

        this.#pongTimer = setTimeout(
            () => this.onPongMiss(seq),
            DEFAULT_OPTIONS.pongTimeoutMs
        );
    }

    onPong(data) {
        if (this.#state !== State.OPEN) return;
        if (String(data ?? '') !== String(this.#pingSeq)) return;

        this.#missedPongs = 0;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;
    }

    onPongMiss(seq) {
        if (this.#state !== State.OPEN) return;
        if (seq !== this.#pingSeq) return;
        if (this.#pongTimer === null) return;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;

        this.#missedPongs += 1;

        if (this.#missedPongs > DEFAULT_OPTIONS.maxMissedPongs) {
            console.debug(
                `closing connection for ${this.#login} after ${this.#missedPongs} missed pongs`
            );
            this.close(HARD_CLOSE);
            return;
        }

        console.debug(
            `missed pong ${this.#missedPongs}/${DEFAULT_OPTIONS.maxMissedPongs} for ${this.#login}`
        );
    }

    sendMatchAndClose(payload, match_auth_token) {
        if (this.#state !== State.OPEN) return;
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.close();
            return;
        }

        try {
            this.#ws.send(JSON.stringify({ ...payload, match_auth_token }));
        } catch (err) {
            console.debug('Failed to send match payload:', err);
        } finally {
            this.close();
        }
    }

    close(code = 1000, reason = '') {
        if (this.#state === State.CLOSED) return;

        this.#state = State.CLOSED;
        this.cleanup();
        if (code === HARD_CLOSE) {
            this.#ws.terminate();
        } else {
            this.#ws.close(code, reason);
        }
        this.emit('close');
    }

    cleanup() {
        clearInterval(this.#pingTimer);
        clearTimeout(this.#pongTimer);

        this.#pingTimer = null;
        this.#pongTimer = null;

        if (this.#onPongHandler) {
            this.#ws.off?.('pong', this.#onPongHandler);
        }
        if (this.#onCloseHandler) {
            this.#ws.off?.('close', this.#onCloseHandler);
        }
        if (this.#onErrorHandler) {
            this.#ws.off?.('error', this.#onErrorHandler);
        }
    }
}

export function parseCookies(cookieHeader) {
    if (cookieHeader === undefined || cookieHeader === '') return {};

    return Object.fromEntries(
        cookieHeader.split(';').map((part) => {
            const [name, ...rest] = part.trim().split('=');
            return [
                decodeCookieValue(name.trim()),
                decodeCookieValue(rest.join('=').trim()),
            ];
        })
    );
}

function decodeCookieValue(value) {
    try {
        return decodeURIComponent(value);
    } catch {
        return value;
    }
}

export class ConnectionServer {
    #wss;
    #connections = new Map();
    #mutex = new Mutex();
    #state = State.INIT;
    #queueTimer = null;
    #pollTimer = null;
    #httpServer = null;
    #upgradeHandler = null;

    constructor(wss = new WebSocketServer({ noServer: true })) {
        this.#wss = wss;
    }

    get state() {
        return this.#state;
    }

    get connections() {
        return this.#connections;
    }

    get httpServer() {
        return this.#httpServer;
    }

    get upgradeHandler() {
        return this.#upgradeHandler;
    }

    open() {
        if (this.#state !== State.INIT) return;
        this.#state = State.OPEN;

        this.#queueTimer = setInterval(
            () => this.#mutex.runExclusive(() => this.extendQueues()),
            DEFAULT_OPTIONS.queueExtensionIntervalMs
        );

        this.#pollTimer = setInterval(
            () => this.#mutex.runExclusive(() => this.onPull()),
            DEFAULT_OPTIONS.pollIntervalMs
        );

        this.#wss.on('connection', (ws, request) =>
            this.#mutex.runExclusive(() => this.onConnection(ws, request))
        );
    }

    async extendQueues() {
        if (this.#state !== State.OPEN) return;

        const queued = [...this.#connections.values()].filter(
            (connection) => connection.state === State.OPEN
        );
        if (queued.length === 0) return;

        try {
            await extendQueueStatuses(
                queued.map((connection) => ({
                    login: connection.login,
                    websocketId: connection.websocketId,
                })),
                DEFAULT_OPTIONS.queueExtensionMs
            );
        } catch (err) {
            console.debug('failed to extend queue status', err);
        }
    }

    async onPull() {
        if (this.#state !== State.OPEN) return;

        const queued = [...this.#connections.values()].filter(
            (connection) => connection.state === State.OPEN
        );
        if (queued.length === 0) return;

        let statuses;
        try {
            statuses = await getConnectionStatuses(
                queued.map((connection) => connection.login)
            );
        } catch (err) {
            console.debug('failed to pull connection statuses', err);
            return;
        }

        for (const connection of queued) {
            const status = statuses.get(connection.login);
            if (status === undefined) {
                connection.close(4004, 'User no longer exists');
                continue;
            }

            if (status.websocketId !== connection.websocketId) {
                connection.close(4001, 'Replaced by new connection');
                continue;
            }

            if (status.matchId !== null) {
                connection.sendMatchAndClose(
                    {
                        login: connection.login,
                        host: status.host,
                        port: status.port,
                    },
                    status.matchAuthToken
                );
            }
        }
    }

    async onConnection(ws, request) {
        if (this.#state !== State.OPEN) {
            ws.close(1001, 'Server is shutting down');
            return;
        }

        const token = parseCookies(request.headers.cookie).session;
        if (token === undefined) {
            ws.close(4401, 'No session token');
            return;
        }

        let login;
        try {
            login = await getLoginFromToken(token);
        } catch (err) {
            console.debug('failed to authenticate connection', err);
            ws.close(1011, 'Internal error');
            return;
        }

        if (login === null) {
            ws.close(4401, 'Invalid session token');
            return;
        }

        const websocketId = randomUUID();
        let registered;
        try {
            registered = await setUserWebsocket(
                login,
                websocketId,
                DEFAULT_OPTIONS.queueExtensionMs
            );
        } catch (err) {
            console.debug('failed to register websocket', err);
            ws.close(1011, 'Internal error');
            return;
        }

        if (!registered) {
            ws.close(4000, 'Already in match');
            return;
        }

        const existingConnection = this.#connections.get(login);
        if (existingConnection !== undefined) {
            existingConnection.close(4001, 'Replaced by new connection');
        }

        const connection = new Connection(ws, login, websocketId);
        connection.on('close', () => this.closeConnection(connection));
        this.#connections.set(login, connection);
        connection.open();
    }

    closeConnection(connection) {
        if (this.#connections.get(connection.login) !== connection) return;
        this.#connections.delete(connection.login);
        void removeQueueStatus(connection.login, connection.websocketId).catch(
            (err) => console.debug('failed to remove queue status', err)
        );
    }

    async close() {
        await this.#mutex.runExclusive(async () => {
            if (this.#state === State.CLOSED) return;
            this.#state = State.CLOSED;

            clearInterval(this.#queueTimer);
            this.#queueTimer = null;

            clearInterval(this.#pollTimer);
            this.#pollTimer = null;

            for (const connection of [...this.#connections.values()]) {
                connection.close();
            }
            this.#connections.clear();

            this.detach();
            this.#wss.close?.();
        });
    }

    attach(httpServer) {
        if (this.#httpServer !== null) return;

        this.#httpServer = httpServer;
        this.#upgradeHandler = (request, socket, head) => {
            let pathname;
            try {
                ({ pathname } = new URL(request.url, 'http://localhost'));
            } catch {
                socket.destroy();
                return;
            }
            if (pathname !== DEFAULT_OPTIONS.connectionPath) return;

            this.#wss.handleUpgrade(request, socket, head, (ws) => {
                this.#wss.emit('connection', ws, request);
            });
        };
        this.#httpServer.on('upgrade', this.#upgradeHandler);
    }

    detach() {
        if (this.#httpServer === null || this.#upgradeHandler === null) return;

        this.#httpServer.off('upgrade', this.#upgradeHandler);
        this.#httpServer = null;
        this.#upgradeHandler = null;
    }
}

export async function createWebSocketServer(httpServer) {
    const server = new ConnectionServer();
    await server.open();
    server.attach(httpServer);
    return server;
}
