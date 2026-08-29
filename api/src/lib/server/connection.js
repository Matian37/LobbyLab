import { EventEmitter } from 'node:events';
import { WebSocket } from 'ws';
import { CONNECTION_ERRORS } from './../errors.js';
import { CONNECTION_DEFAULT_OPTIONS, State } from './../constants.js';
import { websocketLogger } from '../logger.js';

/**
 * Sentinel passed to {@link Connection.close} to request a hard close, which
 * terminates the underlying TCP connection instead of sending a close frame.
 */
const HARD_CLOSE = Symbol('hard close');

/**
 * Wraps a single WebSocket with its own {@link State}.
 *
 * Lifecycle: a connection is `INIT` on construction, transitions to `OPEN` via
 * {@link Connection.open} (which starts the ping/pong watchdog), and to
 * `CLOSED` via {@link Connection.close}, either soft (close frame) or hard
 * (`terminate`). Calls that are invalid for the current state are ignored or
 * throw a {@link ConnectionStateError}.
 *
 * While open, a ping control frame carrying an incrementing sequence is sent
 * every `pingIntervalMs`. A matching pong must arrive within `pongTimeoutMs`,
 * otherwise the pong counts as missed. After more than `maxMissedPongs`
 * consecutive misses the connection is hard-closed. When a match is assigned,
 * {@link Connection.sendMatchAndClose} delivers the payload and closes.
 *
 * @emits close when the connection has been closed.
 */
export class Connection extends EventEmitter {
    #ws;
    #websocketId;

    #state = State.INIT;

    #login;
    #options;

    #logger;

    #pingTimer = null;
    #pongTimer = null;

    #pingSeq = 0;
    #missedPongs = 0;
    #waitingForPing = false;

    #onPongHandler = null;
    #onCloseHandler = null;
    #onErrorHandler = null;

    /**
     * @param {import('ws').WebSocket} ws The underlying socket.
     * @param {string} login The authenticated user's login.
     * @param {number} websocketId Allocated websocket ID for this connection.
     * @param {import('../constants.js').ConnectionOptions} [options] Overrides
     *     for the ping/pong watchdog settings.
     */
    constructor(ws, login, websocketId, options = CONNECTION_DEFAULT_OPTIONS) {
        super();
        this.#ws = ws;
        this.#websocketId = websocketId;
        this.#login = login;
        this.#options = options;
        this.#logger = websocketLogger.child({
            class: 'connection',
            login,
            websocketId,
        });
    }

    /**
     * @returns {import('../constants.js').ConnectionOptions} The watchdog options.
     *
     * Exposed for testing only.
     */
    get options() {
        return this.#options;
    }

    /**
     * @returns {number} The allocated websocket ID for this connection.
     */
    get websocketId() {
        return this.#websocketId;
    }

    /**
     * @returns {string} The authenticated user's login.
     */
    get login() {
        return this.#login;
    }

    /**
     * @returns {import('../constants.js').State} The current lifecycle state.
     */
    get state() {
        return this.#state;
    }

    /**
     * @returns {number} Number of consecutive pongs missed.
     *
     * Exposed for testing only.
     */
    get missedPongs() {
        return this.#missedPongs;
    }

    /**
     * @returns {NodeJS.Timeout|null} The active ping interval, or `null`.
     *
     * Exposed for testing only.
     */
    get pingTimer() {
        return this.#pingTimer;
    }

    /**
     * @returns {NodeJS.Timeout|null} The active pong deadline timer, or `null`.
     *
     * Exposed for testing only.
     */
    get pongTimer() {
        return this.#pongTimer;
    }

    /**
     * @returns {boolean} `true` while a ping is outstanding and a pong has not
     *     yet been received or missed.
     *
     * Exposed for testing only.
     */
    get waitingForPing() {
        return this.#waitingForPing;
    }

    /**
     * Opens the connection: attaches socket listeners and starts the periodic
     * ping watchdog. The state must be `INIT`, otherwise a
     * {@link ConnectionStateError} is thrown from
     * {@link CONNECTION_ERRORS.cannotOpenConnection}.
     *
     * If the underlying socket is already closed, the connection is closed
     * immediately instead of being opened.
     *
     * @throws {Error} When the connection is not in the `INIT` state.
     */
    open() {
        this.#logger.debug('opening connection');

        if (this.#state !== State.INIT) {
            throw CONNECTION_ERRORS.cannotOpenConnection(this.#state);
        }
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.#logger.debug(
                { readyState: this.#ws.readyState },
                'connection socket not open, closing'
            );
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
            this.#options.pingIntervalMs
        );

        this.#logger.debug('connection opened');
    }

    /**
     * Sends the next ping control frame and setups the pong deadline. Ping is
     * skipped when the connection is not open, the socket is closed or
     * a pong from a previous ping is still pending.
     */
    sendPing() {
        this.#logger.debug({ pingSeq: this.#pingSeq }, 'sending ping');

        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'skipping ping, connection not open'
            );
            return;
        }
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.#logger.debug(
                { readyState: this.#ws.readyState },
                'connection socket not open, closing'
            );
            this.close();
            return;
        }
        if (this.#waitingForPing) {
            this.#logger.error(
                'skipping ping, still waiting for previous one; ping interval may be too short'
            );
            return;
        }

        this.#waitingForPing = true;
        try {
            this.#ws.ping(String(this.#pingSeq));
        } catch (err) {
            this.#logger.warn({ err }, 'failed to send ping');
            this.close();
            return;
        }

        this.#pongTimer = setTimeout(
            () => this.onPongMiss(this.#pingSeq),
            this.#options.pongTimeoutMs
        );
    }

    /**
     * Handles an incoming pong. The pong is accepted only while the connection
     * is open, ping is still waiting, and the payload matches the expected
     * sequence. On accept, the missed-pong counter resets and the sequence
     * advances.
     *
     * @param {import('ws').RawData} data Pong payload.
     */
    onPong(data) {
        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'ignoring pong, connection not open'
            );
            return;
        }
        if (!this.#waitingForPing) {
            this.#logger.debug('ignoring pong, no ping waiting');
            return;
        }
        if (String(data ?? '') !== String(this.#pingSeq)) {
            this.#logger.debug(
                { pingSeq: this.#pingSeq, data },
                'ignoring pong, wrong sequence'
            );
            return;
        }

        this.#logger.debug({ pingSeq: this.#pingSeq }, 'pong received');

        this.#waitingForPing = false;
        this.#missedPongs = 0;
        this.#pingSeq++;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;
    }

    /**
     * Records that if a pong was not received in time it is considered a
     * missed pong. A miss is counted only for the currently pending sequence
     * number and it increments the `missedPongs` counter. Once consecutive
     * misses exceed `maxMissedPongs`, the connection is hard-closed.
     *
     * @param {number} seq The sequence whose pong deadline elapsed.
     */
    onPongMiss(seq) {
        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'ignoring missed pong, connection not open'
            );
            return;
        }
        if (!this.#waitingForPing) {
            this.#logger.debug('ignoring missed pong, no ping waiting');
            return;
        }
        if (seq !== this.#pingSeq) {
            this.#logger.debug(
                { seq, pingSeq: this.#pingSeq },
                'ignoring missed pong, wrong sequence'
            );
            return;
        }

        this.#logger.debug({ pingSeq: this.#pingSeq }, 'pong missed');

        this.#waitingForPing = false;
        this.#missedPongs++;
        this.#pingSeq++;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;

        if (this.#missedPongs > this.#options.maxMissedPongs) {
            this.#logger.warn('closing connection after too many missed pongs');
            this.close(HARD_CLOSE);
            return;
        }

        this.#logger.debug(
            { missedPongs: this.#missedPongs },
            'missed pong counted'
        );
    }

    /**
     * Sends the match-assignment payload as a JSON text frame and then closes
     * the connection normally.
     *
     * @param {{ login: string, host: string, port: string, matchAuthToken: string }} payload
     *     Match details to deliver to the client.
     */
    sendMatchAndClose(payload) {
        this.#logger.debug('sending match and closing connection');

        if (this.#state !== State.OPEN) {
            this.#logger.debug(
                { state: this.#state },
                'skipping match send, connection not open'
            );
            return;
        }
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.#logger.debug(
                { readyState: this.#ws.readyState },
                'connection socket not open, closing'
            );
            this.close();
            return;
        }

        try {
            // FIX: should wait for send to complete before closing
            this.#ws.send(JSON.stringify(payload));
            this.#logger.debug(
                { host: payload.host, port: payload.port },
                'match payload sent to connection'
            );
        } catch (err) {
            this.#logger.warn({ err }, 'failed to send match payload');
        } finally {
            this.close();
        }
    }

    /**
     * Closes the connection. A soft close sends a WebSocket close frame with
     * the given code and reason; a hard close (passing {@link HARD_CLOSE})
     * terminates the underlying TCP connection without a close frame. The
     * state becomes `CLOSED`, timers and listeners are cleaned up, and a
     * `close` event is emitted. Repeated calls after closing are ignored.
     *
     * @param {number|symbol} [code=1000] Close code, or {@link HARD_CLOSE} to
     *     terminate.
     * @param {string} [reason=''] Close reason included in the close frame.
     */
    close(code = 1000, reason = '') {
        this.#logger.debug({ code, reason }, 'closing connection');

        if (this.#state === State.CLOSED) {
            this.#logger.debug('ignoring close, connection already closed');
            return;
        }

        this.#state = State.CLOSED;
        this.cleanup();
        if (code === HARD_CLOSE) {
            this.#logger.debug('hard close, terminating connection');
            this.#ws.terminate();
        } else {
            this.#logger.debug(
                { code, reason },
                'soft close, closing connection'
            );
            this.#ws.close(code, reason);
        }
        this.emit('close');

        this.#logger.debug('connection closed');
    }

    /**
     * Stops timers and detaches socket listeners so the connection no longer
     * responds to the socket after it is closed.
     */
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
