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

/**
 * Holds the {@link Connection} instances for all accepted websockets, keyed by
 * login. Ensures only the newest websocket for a login is tracked: a stale
 * connection is closed with `4001 "Replaced by new connection"` and replaced.
 */
export class Connections {
    #connections = new Map();
    #logger = websocketLogger.child({ class: 'connections' });

    /**
     * Registers a connection, evicting any older connection for the same login.
     * The incoming connection is rejected (closed immediately) if a strictly
     * newer websocket for that login is already registered.
     *
     * @param {Connection} connection The connection to register.
     */
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

    /**
     * Removes a connection from tracking. The connection is removed only if the
     * stored websocket ID matches, so a stale close cannot evict a newer
     * connection. Queue status is cleared in the database asynchronously.
     *
     * @param {string} login The user's login.
     * @param {number} websocketId The websocket ID of the connection to remove.
     */
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

    /**
     * @returns {Connection[]} All tracked connections currently in the `OPEN`
     *     state (i.e. those still in the matchmaking queue).
     */
    getQueued() {
        return [...this.#connections.values()].filter(
            (connection) => connection.state === State.OPEN
        );
    }

    /**
     * Closes and removes every tracked connection.
     */
    close() {
        this.#logger.debug('closing connections');

        for (const connection of this.#connections.values()) {
            connection.close();
        }
        this.#connections.clear();

        this.#logger.debug('all connections closed');
    }
}

/**
 * Registers a user for matchmaking and creates the {@link Connection} that will
 * track their websocket until a match is assigned.
 *
 * @param {import('ws').WebSocket} ws The accepted websocket.
 * @param {string} login The authenticated user's login.
 * @param {import('../constants.js').ServerOptions} options Server options that
 *     carry the queue deadline (`queueExtensionMs`).
 * @returns {Promise<Connection|null>} The new connection, or `null` when the
 *     user could not be queued (already in a match, or a database error).
 */
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

/**
 * Extracts and validates the session token from the websocket handshake
 * request's `Cookie` header.
 *
 * @param {import('http').IncomingMessage} request The upgrade request.
 * @returns {{ error: string, code: number } | { data: string }} `{ data }` with
 *     the token on success, otherwise `{ error }` with the rejection reason and
 *     close code (`4401` for all auth failures).
 */
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

/**
 * Destroys a rejected upgrade socket. In production the socket is destroyed
 * unconditionally; in development, sockets whose `Sec-WebSocket-Protocol`
 * header starts with `vite` are left alone so Vite's HMR client does not break.
 *
 * @param {import('http').IncomingMessage} request The upgrade request.
 * @param {import('node:net').Socket} socket The socket to destroy.
 */
export function safeSocketDestroy(request, socket) {
    if (process.env.NODE_ENV === 'production') {
        socket.destroy();
        return;
    }

    if (request.headers['sec-websocket-protocol']?.startsWith('vite')) return;

    socket.destroy();
}

/**
 * Runs the matchmaking WebSocket endpoint: authenticates handshakes, tracks
 * queued connections, keeps their queue deadlines alive, and delivers the match
 * assignment to the right connection once one is available.
 *
 * Lifecycle mirrors {@link State}: `INIT` on construction, `OPEN` via
 * {@link ConnectionServer.open} (which starts the two background loops and
 * attaches to the HTTP `upgrade` event), and `CLOSED` via
 * {@link ConnectionServer.close}.
 *
 * While open, two timers drive the logic:
 *   - the queue-extension timer calls {@link ConnectionServer.extendQueues} so
 *     queued users do not time out while waiting;
 *   - the poll timer calls {@link ConnectionServer.onPull} to look up the
 *     current database status of each queued connection and react to it.
 */
export class ConnectionServer {
    #wss;
    #options;
    #connections = new Connections();
    #logger = websocketLogger.child({ class: 'server' });
    #state = State.INIT;
    #queueTimer = null;
    #pollTimer = null;
    #httpServer = null;

    /**
     * @param {import('http').Server} httpServer The HTTP server whose `upgrade`
     *     events this server handles.
     * @param {WebSocketServer} [wss] Underlying server. Defaults to a
     *     `noServer` instance managed entirely by this class; tests pass a
     *     mocked instance instead.
     * @param {import('../constants.js').ServerOptions} [options] Tuning options
     *     for timers, the connection path, and per-connection watchdog.
     */
    constructor(
        httpServer,
        wss = new WebSocketServer({ noServer: true }),
        options = SERVER_DEFAULT_OPTIONS
    ) {
        this.#httpServer = httpServer;
        this.#wss = wss;
        this.#options = options;
    }

    /**
     * @returns {import('../constants.js').ServerOptions} The server options.
     *
     * Exposed for testing only.
     */
    get options() {
        return this.#options;
    }

    /**
     * @returns {import('../constants.js').State} The current lifecycle state.
     *
     * Exposed for testing only.
     */
    get state() {
        return this.#state;
    }

    /**
     * @returns {Connections} The tracked connections registry.
     *
     * Exposed for testing only.
     */
    get connections() {
        return this.#connections;
    }

    /**
     * @returns {import('http').Server|null} The attached HTTP server, or `null`
     *     after the server is closed.
     *
     * Exposed for testing only.
     */
    get httpServer() {
        return this.#httpServer;
    }

    /**
     * Opens the server: starts the queue-extension and status-poll loops and
     * attaches the `upgrade` handler to the HTTP server. The state must be
     * `INIT`, otherwise a {@link ConnectionStateError} is thrown from
     * {@link CONNECTION_ERRORS.cannotOpenServer}.
     *
     * @throws {Error} When the server is not in the `INIT` state.
     */
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

    /**
     * Handles an HTTP `upgrade` request. Requests whose URL does not match the
     * configured connection path are rejected; matching requests are handed off
     * to the WebSocket server, which emits a `connection` event once upgraded.
     */
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
            safeSocketDestroy(request, socket);
            return;
        }

        this.#wss.handleUpgrade(request, socket, head, (ws) => {
            this.#wss.emit('connection', ws, request);
        });
    };

    /**
     * Extends the queue deadline of every currently queued connection so their
     * matchmaking entries do not expire while they wait.
     *
     * @returns {Promise<void>}
     */
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

    /**
     * Polls the database for the current status of each queued connection and
     * reacts to:
     *   - a login that no longer exists closes the connection (`4004`);
     *   - a connection whose websocket ID is out of date is replaced (`4001`);
     *   - a connection with no match yet stays queued;
     *   - a connection with a match is delivered its match details and closed.
     *
     * @returns {Promise<void>}
     */
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

    /**
     * Handles a newly upgraded websocket: authenticates the session cookie,
     * registers the user in the matchmaking queue, and tracks the connection.
     * Connections that fail any step are closed with the appropriate close code
     * and reason.
     *
     * @param {import('ws').WebSocket} ws The upgraded websocket.
     * @param {import('http').IncomingMessage} request The upgrade request.
     * @returns {Promise<void>}
     */
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

        // Prevents adding websocket to connections
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

    /**
     * Closes the server: stops both timers, detaches the `upgrade` handler,
     * closes all tracked connections and the underlying WebSocket server.
     * Repeated calls after closing are ignored.
     */
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

/**
 * Convenience factory that creates, opens, and returns a {@link ConnectionServer}
 * attached to the given HTTP server.
 *
 * @param {import('http').Server} httpServer The HTTP server to attach to.
 * @param {import('../constants.js').ServerOptions} [options] Server options.
 * @returns {Promise<ConnectionServer>} The opened connection server.
 */
export async function createWebSocketServer(httpServer, options) {
    const server = new ConnectionServer(httpServer, undefined, options);
    server.open();
    return server;
}
