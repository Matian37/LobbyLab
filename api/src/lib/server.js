import { WebSocketServer } from 'ws';
import { parse } from 'cookie-es';
import {
    getLoginFromToken,
    extendQueueStatuses,
    removeQueueStatus,
    setQueueStatus,
    getConnectionStatuses,
} from './db.js';
import { isValidToken } from './validate.js';
import { CONNECTION_ERRORS } from './errors.js';
import { SERVER_DEFAULT_OPTIONS, State } from './constants.js';
import { Connection } from './connection.js';

export class Connections {
    #connections = new Map();

    set(connection) {
        const existingConnection = this.#connections.get(connection.login);

        if (
            existingConnection !== undefined &&
            existingConnection.websocketId > connection.websocketId
        ) {
            connection.close(4001, 'Replaced by new connection');
            return;
        }

        this.#connections.delete(connection.login);
        this.#connections.set(connection.login, connection);
        connection.on('close', () =>
            this.delete(connection.login, connection.websocketId)
        );

        if (existingConnection !== undefined) {
            existingConnection.close(4001, 'Replaced by new connection');
        }

        connection.open();
    }

    delete(login, websocketId) {
        const connection = this.#connections.get(login);

        if (connection === undefined) return;
        if (connection.websocketId !== websocketId) return;

        this.#connections.delete(login);
        connection.close();
        removeQueueStatus(login, websocketId).catch((err) =>
            console.debug('failed to remove queue status', err)
        );
    }

    getQueued() {
        return [...this.#connections.values()].filter(
            (connection) => connection.state === State.OPEN
        );
    }

    close() {
        for (const connection of this.#connections.values()) {
            connection.close();
        }
        this.#connections.clear();
    }
}

async function createConnection(ws, login, options) {
    let websocketId;
    try {
        websocketId = await setQueueStatus(login, options.queueExtensionMs);
    } catch (err) {
        console.debug('failed to register websocket', err);
        ws.close(1011, 'Internal error');
        return null;
    }

    if (websocketId === null) {
        ws.close(4000, 'Already in match');
        return null;
    }

    return new Connection(ws, login, websocketId, options);
}

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
    #connections = new Connections();
    #state = State.INIT;
    #queueTimer = null;
    #pollTimer = null;
    #httpServer = null;

    constructor(
        httpServer,
        wss = new WebSocketServer({ noServer: true }),
        options = SERVER_DEFAULT_OPTIONS
    ) {
        this.#httpServer = httpServer;
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

    open() {
        if (this.#state !== State.INIT) {
            throw CONNECTION_ERRORS.cannotOpenServer(this.#state);
        }
        this.#state = State.OPEN;

        this.#queueTimer = setInterval(
            () => this.extendQueues(),
            this.#options.queueExtensionIntervalMs
        );

        this.#pollTimer = setInterval(
            () => this.onPull(),
            this.#options.pollIntervalMs
        );

        this.#wss.on('connection', (ws, request) =>
            this.onConnection(ws, request)
        );

        this.#httpServer.on('upgrade', this.#upgradeHandler);
    }

    #upgradeHandler = (request, socket, head) => {
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

    async extendQueues() {
        if (this.#state !== State.OPEN) return;

        const queued = this.#connections.getQueued();

        await extendQueueStatuses(queued, this.#options.queueExtensionMs).catch(
            (err) => console.debug('failed to extend queue status', err)
        );
    }

    async onPull() {
        if (this.#state !== State.OPEN) return;

        const queued = this.#connections.getQueued();

        let statuses;
        try {
            statuses = await getConnectionStatuses(queued);
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

        const connection = await createConnection(ws, login, this.#options);
        if (connection === null) return;

        // prevent adding websocket to connections
        // when close was already called during await
        if (this.#state !== State.OPEN) {
            ws.close(1001, 'Server is shutting down');
            return;
        }

        this.#connections.set(connection);
    }

    close() {
        if (this.#state === State.CLOSED) return;
        this.#state = State.CLOSED;

        clearInterval(this.#queueTimer);
        this.#queueTimer = null;

        clearInterval(this.#pollTimer);
        this.#pollTimer = null;

        this.#httpServer.off('upgrade', this.#upgradeHandler);
        this.#httpServer = null;

        this.#connections.close();

        this.#wss.close?.();
    }
}

export async function createWebSocketServer(httpServer, options) {
    const server = new ConnectionServer(httpServer, undefined, options);
    server.open();
    return server;
}
