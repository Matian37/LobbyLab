import { EventEmitter } from 'node:events';
import { randomUUID } from 'node:crypto';
import { WebSocket } from 'ws';
import { CONNECTION_ERRORS } from './errors.js';

export const DEFAULT_OPTIONS = Object.freeze({
    connectionPath: '/api/connection',
    pingIntervalMs: 5_000,
    pongTimeoutMs: 3_000,
    maxMissedPongs: 2,
    queueExtensionIntervalMs: 3_000,
    queueExtensionMs: 5_000,
    pollIntervalMs: 2_500,
});

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
    #options;
    #state = State.INIT;
    #missedPongs = 0;
    #pingTimer = null;
    #pongTimer = null;
    #pingSeq = 0;
    #waitingForPing = false;
    #onPongHandler = null;
    #onCloseHandler = null;
    #onErrorHandler = null;

    constructor(
        ws,
        login,
        websocketId = randomUUID(),
        options = DEFAULT_OPTIONS
    ) {
        super();
        this.#ws = ws;
        this.#websocketId = websocketId;
        this.#login = login;
        this.#options = options;
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
        if (this.#state !== State.INIT) {
            throw CONNECTION_ERRORS.cannotOpenConnection(this.#state);
        }
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
            this.#options.pingIntervalMs
        );
    }

    sendPing() {
        if (this.#state !== State.OPEN) return;
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.close();
            return;
        }
        if (this.#waitingForPing) {
            console.warn('skipping sending new ping, still waiting for pong');
            return;
        }

        this.#waitingForPing = true;
        try {
            this.#ws.ping(String(this.#pingSeq));
        } catch (err) {
            console.debug('failed to send ping:', err);
            this.close();
            return;
        }

        this.#pongTimer = setTimeout(
            () => this.onPongMiss(this.#pingSeq),
            this.#options.pongTimeoutMs
        );
    }

    onPong(data) {
        if (this.#state !== State.OPEN) return;
        if (!this.#waitingForPing) return;
        if (String(data ?? '') !== String(this.#pingSeq)) return;

        this.#waitingForPing = false;
        this.#missedPongs = 0;
        this.#pingSeq++;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;
    }

    onPongMiss(seq) {
        if (this.#state !== State.OPEN) return;
        if (!this.#waitingForPing) return;
        if (seq !== this.#pingSeq) return;

        this.#waitingForPing = false;
        this.#missedPongs++;
        this.#pingSeq++;

        clearTimeout(this.#pongTimer);
        this.#pongTimer = null;

        if (this.#missedPongs > this.#options.maxMissedPongs) {
            console.debug(
                `closing connection for ${this.#login} after ${this.#missedPongs} missed pongs`
            );
            this.close(HARD_CLOSE);
            return;
        }

        console.debug(
            `missed pong ${this.#missedPongs}/${this.#options.maxMissedPongs} for ${this.#login}`
        );
    }

    sendMatchAndClose(payload) {
        if (this.#state !== State.OPEN) return;
        if (this.#ws.readyState !== WebSocket.OPEN) {
            this.close();
            return;
        }

        try {
            this.#ws.send(JSON.stringify(payload));
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
