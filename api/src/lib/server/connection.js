import { EventEmitter } from 'node:events';
import { WebSocket } from 'ws';
import { CONNECTION_ERRORS } from './../errors.js';
import { CONNECTION_DEFAULT_OPTIONS, State } from './../constants.js';
import { websocketLogger } from '../logger.js';

const HARD_CLOSE = Symbol('hard close');

export class Connection extends EventEmitter {
    #ws;
    #websocketId;
    #login;
    #options;
    #logger;
    #state = State.INIT;
    #missedPongs = 0;
    #pingTimer = null;
    #pongTimer = null;
    #pingSeq = 0;
    #waitingForPing = false;
    #onPongHandler = null;
    #onCloseHandler = null;
    #onErrorHandler = null;

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

    get options() {
        return this.#options;
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

    get pongTimer() {
        return this.#pongTimer;
    }

    get waitingForPing() {
        return this.#waitingForPing;
    }

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
            // TODO: should wait for send to complete before closing
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
