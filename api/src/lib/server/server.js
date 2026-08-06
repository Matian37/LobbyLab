import { WebSocketServer } from 'ws';
import { parse } from 'cookie-es';
import {
    getLoginFromToken,
    extendQueueStatuses,
    removeQueueStatus,
    setQueueStatus,
    getConnectionStatuses,
} from './../db.js';
import { isValidToken } from './../validate.js';
import { CONNECTION_ERRORS } from './../errors.js';
import { SERVER_DEFAULT_OPTIONS, State } from './../constants.js';
import { Connection } from './connection.js';
import { websocketLogger } from '../logger.js';

export class Connections {
    #connections = new Map();
    #logger = websocketLogger.child({ class: 'connections' });

    set(connection) {
        const existingConnection = this.#connections.get(connection.login);

        if (
            existingConnection !== undefined &&
            existingConnection.websocketId > connection.websocketId
        ) {
            this.#logger.info(
                {
                    login: connection.login,
                    existingWebsocketId: existingConnection.websocketId,
                    incomingWebsocketId: connection.websocketId,
                },
                'rejecting connection, newer websocket already connected'
            );
            connection.close(4001, 'Replaced by new connection');
            return;
        }

        this.#connections.delete(connection.login);
        this.#connections.set(connection.login, connection);
        connection.on('close', () =>
            this.delete(connection.login, connection.websocketId)
        );

        if (existingConnection !== undefined) {
            this.#logger.info(
                {
                    login: connection.login,
                    existingWebsocketId: existingConnection.websocketId,
                    incomingWebsocketId: connection.websocketId,
                },
                'existing connection replaced by a newer websocket'
            );
            existingConnection.close(4001, 'Replaced by new connection');
        }

        this.#logger.debug(
            {
                login: connection.login,
                websocketId: connection.websocketId,
            },
            'connection registered'
        );
        connection.open();
    }

    delete(login, websocketId) {
        const connection = this.#connections.get(login);

        if (connection === undefined) return;
        if (connection.websocketId !== websocketId) return;

        this.#connections.delete(login);
        connection.close();
        this.#logger.info(
            { login, websocketId },
            'connection closed and removed from tracking'
        );
        removeQueueStatus(login, websocketId).catch((err) =>
            this.#logger.error(
                { login, websocketId, err },
                'failed to remove queue status'
            )
        );
    }

    getQueued() {
        return [...this.#connections.values()].filter(
            (connection) => connection.state === State.OPEN
        );
    }

    close() {
        this.#logger.debug('closing connections');

        for (const connection of this.#connections.values()) {
            connection.close();
        }
        this.#connections.clear();

        this.#logger.debug('all connections closed');
    }
}

export async function createConnection(ws, login, options) {
    let websocketId;
    try {
        websocketId = await setQueueStatus(login, options.queueExtensionMs);
        websocketLogger.debug(
            { login, websocketId },
            'queue status registered for connection'
        );
    } catch (err) {
        websocketLogger.error({ login, err }, 'failed to register websocket');
        ws.close(1011, 'Internal error');
        return null;
    }

    if (websocketId === null) {
        websocketLogger.info(
            { login },
            'user is already in a match, rejecting connection'
        );
        ws.close(4000, 'Already in match');
        return null;
    }

    return new Connection(ws, login, websocketId, options);
}

export function parseSessionToken(request) {
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
    #logger = websocketLogger.child({ class: 'server' });
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
        this.#logger.debug('opening connection server');

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

        this.#logger.info(
            { connectionPath: this.#options.connectionPath },
            'connection server opened'
        );
    }

    #upgradeHandler = (request, socket, head) => {
        this.#logger.debug(
            {
                remoteAddress: socket.remoteAddress,
                remotePort: socket.remotePort,
                url: request.url,
            },
            'handling websocket upgrade'
        );

        let pathname;
        try {
            ({ pathname } = new URL(request.url, 'http://localhost'));
        } catch {
            this.#logger.info(
                {
                    remoteAddress: socket.remoteAddress,
                    remotePort: socket.remotePort,
                },
                'rejecting websocket upgrade, invalid url'
            );
            socket.destroy();
            return;
        }
        if (pathname !== this.#options.connectionPath) {
            this.#logger.info(
                {
                    remoteAddress: socket.remoteAddress,
                    remotePort: socket.remotePort,
                },
                'rejecting websocket upgrade, path does not match'
            );
            socket.destroy();
            return;
        }

        this.#wss.handleUpgrade(request, socket, head, (ws) => {
            this.#wss.emit('connection', ws, request);
        });
    };

    async extendQueues() {
        this.#logger.debug('extending queues');

        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'skipping queue extension, server not open'
            );
            return;
        }

        const queued = this.#connections.getQueued();

        await extendQueueStatuses(queued, this.#options.queueExtensionMs).catch(
            (err) =>
                this.#logger.error({ err }, 'failed to extend queue status')
        );

        this.#logger.debug(
            {
                websocketIds: queued.map(
                    (connection) => connection.websocketId
                ),
            },
            'queue extension finished'
        );
    }

    async onPull() {
        this.#logger.debug('pulling connection statuses');

        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'skipping status pull, server not open'
            );
            return;
        }

        const queued = this.#connections.getQueued();

        let statuses;
        try {
            statuses = await getConnectionStatuses(queued);
        } catch (err) {
            this.#logger.error({ err }, 'failed to pull connection statuses');
            return;
        }

        for (const connection of queued) {
            const status = statuses.get(connection.login);
            if (status === undefined) {
                this.#logger.warn(
                    { login: connection.login },
                    'user no longer exists, closing connection'
                );
                connection.close(4004, 'User no longer exists');
                continue;
            }

            if (status.websocketId !== connection.websocketId) {
                this.#logger.info(
                    {
                        login: connection.login,
                        oldWebsocketId: connection.websocketId,
                        newWebsocketId: status.websocketId,
                    },
                    'connection replaced according to database state, closing it'
                );
                connection.close(4001, 'Replaced by new connection');
                continue;
            }

            if (status.matchId === null) {
                this.#logger.debug(
                    {
                        login: connection.login,
                        websocketId: connection.websocketId,
                    },
                    'no match assigned yet, keeping connection queued'
                );
                continue;
            }

            this.#logger.info(
                {
                    login: connection.login,
                    websocketId: connection.websocketId,
                    matchId: status.matchId,
                },
                'match assigned, sending match details'
            );
            connection.sendMatchAndClose({
                login: connection.login,
                host: status.host,
                port: status.port,
                matchAuthToken: status.matchAuthToken,
            });
        }

        this.#logger.debug('status pull finished');
    }

    async onConnection(ws, request) {
        this.#logger.debug('handling new connection');

        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'rejecting connection, server not open'
            );
            ws.close(1001, 'Server is shutting down');
            return;
        }

        const result = parseSessionToken(request);
        if (result.error !== undefined) {
            this.#logger.info(
                {
                    code: result.code,
                    error: result.error,
                    remoteAddress: request.socket?.remoteAddress,
                    remotePort: request.socket?.remotePort,
                },
                'rejecting connection, session token invalid'
            );
            ws.close(result.code, result.error);
            return;
        }
        const token = result.data;

        let login;
        try {
            login = await getLoginFromToken(token);
        } catch (err) {
            this.#logger.error({ err }, 'failed to authenticate connection');
            ws.close(1011, 'Internal error');
            return;
        }
        if (login === null) {
            this.#logger.debug(
                {
                    code: result.code,
                    error: result.error,
                    remoteAddress: request.socket?.remoteAddress,
                    remotePort: request.socket?.remotePort,
                },
                'connection rejected, invalid session token'
            );
            ws.close(4401, 'Invalid session token');
            return;
        }
        this.#logger.debug(
            {
                login,
                remoteAddress: request.socket?.remoteAddress,
                remotePort: request.socket?.remotePort,
            },
            'connection authenticated'
        );

        const connection = await createConnection(ws, login, this.#options);
        if (connection === null) return;

        // prevent adding websocket to connections
        // when close was already called during await
        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'rejecting connection, server closed during creation'
            );
            ws.close(1001, 'Server is shutting down');
            return;
        }

        this.#connections.set(connection);

        if (connection.state == State.OPEN)
            this.#logger.info(
                {
                    login,
                    websocketId: connection.websocketId,
                    remoteAddress: request.socket?.remoteAddress,
                    remotePort: request.socket?.remotePort,
                },
                'connection accepted'
            );
    }

    close() {
        this.#logger.info('closing connection server');

        if (this.#state === State.CLOSED) {
            this.#logger.debug('ignoring close, server already closed');
            return;
        }
        this.#state = State.CLOSED;

        clearInterval(this.#queueTimer);
        this.#queueTimer = null;
        this.#logger.debug('queue extension timer stopped');

        clearInterval(this.#pollTimer);
        this.#pollTimer = null;
        this.#logger.debug('status poll timer stopped');

        this.#httpServer.off('upgrade', this.#upgradeHandler);
        this.#httpServer = null;
        this.#logger.debug('upgrade handler detached');

        this.#connections.close();

        this.#wss.close?.();
        this.#logger.debug('websocket server closed');

        this.#logger.info('connection server closed');
    }
}

export async function createWebSocketServer(httpServer, options) {
    const server = new ConnectionServer(httpServer, undefined, options);
    server.open();
    return server;
}
