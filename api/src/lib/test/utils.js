import { expect, vi } from 'vitest';
import { within } from '@testing-library/dom';

/**
 * Asserts that `response` matches the JSON response produced by `errorFactory`.
 * Convenience for checking that a route returned a specific {@link ERRORS}
 * response.
 *
 * @param {Response} response The SvelteKit response to check.
 * @param {() => Response} errorFactory Factory producing the expected error
 *     response.
 */
export async function expectError(response, errorFactory) {
    const err = errorFactory();
    expect(response.status).toBe(err.status);
    expect(await response.json()).toEqual(await err.json());
}

// A mock SvelteKit `cookies` object backed by an in-memory jar. The methods are
// vi.fn() so tests can both drive the jar and assert on the calls.
/**
 * Creates a mock SvelteKit `cookies` object backed by an in-memory jar, so
 * route handlers can be exercised without an HTTP server. Every method is a
 * `vi.fn()`, letting tests drive the jar and assert on calls.
 *
 * @returns {{
 *     get: import('vitest').Mock,
 *     set: import('vitest').Mock,
 *     delete: import('vitest').Mock,
 *     jar: Map<string, string>
 * }}
 */
export function createCookies() {
    const jar = new Map();
    return {
        get: vi.fn((name) => jar.get(name)),
        set: vi.fn((name, value) => jar.set(name, value)),
        delete: vi.fn((name) => jar.delete(name)),
        jar,
    };
}

/**
 * Reads a table element into a matrix of its cell texts (rows of `td`/`th`
 * values), trimming whitespace.
 *
 * @param {HTMLElement|null} tableElement The table to read.
 * @returns {string[][]} Cell texts by row.
 */
export function getTableMatrix(tableElement) {
    if (!tableElement) {
        throw new Error('getTableMatrix requires a valid table element.');
    }

    return within(tableElement)
        .getAllByRole('row')
        .map((row) =>
            [...row.querySelectorAll('td, th')].map(
                (cell) => cell.textContent?.trim() ?? ''
            )
        );
}

// A fake WebSocket that stays in the CONNECTING state until the test opens it,
// used to exercise cancelling matchmaking before the connection is established.
/**
 * A fake WebSocket that stays in the `CONNECTING` state, used to exercise
 * cancelling matchmaking before the connection is established. Every instance
 * is recorded on {@link FakeConnectingWebSocket.instances} for later assertion.
 */
export class FakeConnectingWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static instances = [];

    /**
     * Clears the record of previously created instances.
     */
    static resetInstances() {
        FakeConnectingWebSocket.instances.length = 0;
    }

    /**
     * @param {string} url The URL the connection targets.
     */
    constructor(url) {
        this.url = url;
        this.readyState = FakeConnectingWebSocket.CONNECTING;
        this.closed = false;
        this.onopen = null;
        this.onclose = null;
        this.onerror = null;
        this.onmessage = null;
        FakeConnectingWebSocket.instances.push(this);
    }

    close() {
        this.closed = true;
    }
}
