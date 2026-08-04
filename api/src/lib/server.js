import { randomUUID } from 'node:crypto';
import { WebSocketServer } from 'ws';
import { Mutex } from 'async-mutex';
import { parse } from 'cookie-es';
import {
    getLoginFromToken,
    extendQueueStatuses,
    removeQueueStatus,
    setUserWebsocket,
    getConnectionStatuses,
} from './db.js';
import { isValidToken } from './validate.js';
import { CONNECTION_ERRORS } from './errors.js';
import { SERVER_DEFAULT_OPTIONS, State } from './constants.js';
import { Connection } from './connection.js';

function parseSessionToken(request) {
    const cookie = request.headers.cookie;

    if (cookie === undefined) return { error: 'No cookie', code: 4401 };
    if (cookie.length > 8192)
        return { error: 'Cookie header too large', code: 4401 };

    const session = parse(cookie).session;

    if (session === undefined) return { error: 'No session token', code: 4401 };
    if (!isValidToken(session))
        return { error: 'Invalid session token', code: 4401 };
    return { data: session };
}

export class ConnectionServer {
    #wss;
    #options;
    #connections = new Map();
    #mutex = new Mutex();
    #state = State.INIT;
    #queueTimer = null;
    #pollTimer = null;
    #httpServer = null;
    #upgradeHandler = null;

    constructor(
        wss = new WebSocketServer({ noServer: true }),
        options = SERVER_DEFAULT_OPTIONS
    ) {
        this.#wss = wss;
        this.#options = options;
    }

    get options() {
        return this.#options;
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
        if (this.#state !== State.INIT) {
            throw CONNECTION_ERRORS.cannotOpenServer(this.#state);
        }
        this.#state = State.OPEN;

        this.#queueTimer = setInterval(
            () => this.#mutex.runExclusive(() => this.extendQueues()),
            this.#options.queueExtensionIntervalMs
        );

        this.#pollTimer = setInterval(
            () => this.#mutex.runExclusive(() => this.onPull()),
            this.#options.pollIntervalMs
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

        await extendQueueStatuses(queued, this.#options.queueExtensionMs).catch(
            (err) => console.debug('failed to extend queue status', err)
        );
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

            if (status.matchId === null) continue;

            connection.sendMatchAndClose({
                login: connection.login,
                host: status.host,
                port: status.port,
                matchAuthToken: status.matchAuthToken,
            });
        }
    }

    async onConnection(ws, request) {
        if (this.#state !== State.OPEN) {
            ws.close(1001, 'Server is shutting down');
            return;
        }

        const result = parseSessionToken(request);
        if (result.error !== undefined) {
            ws.close(result.code, result.error);
            return;
        }
        const token = result.data;

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
                this.#options.queueExtensionMs
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

        const connection = new Connection(
            ws,
            login,
            websocketId,
            this.#options
        );
        connection.on('close', () => this.closeConnection(connection));
        this.#connections.set(login, connection);
        connection.open();
    }

    closeConnection(connection) {
        if (this.#connections.get(connection.login) !== connection) return;
        this.#connections.delete(connection.login);
        removeQueueStatus(connection.login, connection.websocketId).catch(
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

            if (this.#httpServer !== null) {
                this.detach();
            }
            this.#wss.close?.();
        });
    }

    attach(httpServer) {
        if (this.#httpServer !== null) {
            throw CONNECTION_ERRORS.alreadyAttached();
        }

        this.#httpServer = httpServer;
        this.#upgradeHandler = (request, socket, head) => {
            let pathname;
            try {
                ({ pathname } = new URL(request.url, 'http://localhost'));
            } catch {
                socket.destroy();
                return;
            }
            if (pathname !== this.#options.connectionPath) {
                socket.destroy();
                return;
            }

            this.#wss.handleUpgrade(request, socket, head, (ws) => {
                this.#wss.emit('connection', ws, request);
            });
        };
        this.#httpServer.on('upgrade', this.#upgradeHandler);
    }

    detach() {
        if (this.#httpServer === null || this.#upgradeHandler === null) {
            throw CONNECTION_ERRORS.notAttached();
        }

        this.#httpServer.off('upgrade', this.#upgradeHandler);
        this.#httpServer = null;
        this.#upgradeHandler = null;
    }
}

export async function createWebSocketServer(httpServer, options) {
    const server = new ConnectionServer(undefined, options);
    await server.open();
    server.attach(httpServer);
    return server;
}
