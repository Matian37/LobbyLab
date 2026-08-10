import { expect, vi } from 'vitest';
import { within } from '@testing-library/dom';

export async function expectError(response, errorFactory) {
    const err = errorFactory();
    expect(response.status).toBe(err.status);
    expect(await response.json()).toEqual(await err.json());
}

// A mock SvelteKit `cookies` object backed by an in-memory jar. The methods are
// vi.fn() so tests can both drive the jar and assert on the calls.
export function createCookies() {
    const jar = new Map();
    return {
        get: vi.fn((name) => jar.get(name)),
        set: vi.fn((name, value) => jar.set(name, value)),
        delete: vi.fn((name) => jar.delete(name)),
        jar,
    };
}

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
export class FakeConnectingWebSocket {
    static CONNECTING = 0;
    static OPEN = 1;
    static instances = [];

    static resetInstances() {
        FakeConnectingWebSocket.instances.length = 0;
    }

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
