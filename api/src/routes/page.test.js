import {
    vi,
    it,
    expect,
    describe,
    beforeEach,
    afterEach,
    onTestFinished,
} from 'vitest';
import { once } from 'node:events';
import { render, screen, fireEvent, waitFor } from '@testing-library/svelte';
import { cleanup } from '@testing-library/svelte';
import userEvent from '@testing-library/user-event';
import '@testing-library/jest-dom/vitest';
import { WebSocket, WebSocketServer } from 'ws';
import { goto, invalidateAll } from '$app/navigation';
import { enhance } from '$app/forms';
import Page from './+page.svelte';
import * as layout from './+layout.server.js';
import * as pageLoad from './+page.server.js';
import * as db from '$lib/db.js';
import { SESSION_TOKEN_LENGTH, LOGIN_MIN_LENGTH } from '$lib/constants.js';
import { enhanceMock } from '$lib/test/forms.js';
import {
    createCookies,
    getTableMatrix,
    FakeConnectingWebSocket,
} from '$lib/test/utils.js';
import getPort from 'get-port';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);
const EXAMPLE_LOGIN = 'a'.repeat(LOGIN_MIN_LENGTH);
const EXAMPLE_MATCH = { host: 'h', port: 'p', matchAuthToken: 'T' };

const mocks = await vi.hoisted(async () => {
    const { writable } = await import('svelte/store');
    return {
        page: writable({ data: null }),
        nextResult: undefined,
    };
});

vi.mock('$app/navigation', () => ({
    goto: vi.fn(),
    invalidateAll: vi.fn(),
}));
vi.mock('$app/paths', () => ({ resolve: (path) => path }));
vi.mock('$app/stores', () => ({ page: mocks.page }));
vi.mock('$app/forms', () => ({ enhance: vi.fn() }));
vi.mock('$env/dynamic/public', () => ({
    env: {
        PUBLIC_GAME_LAUNCH_URL: 'http://game/{host}/{port}/{token}',
    },
}));
vi.mock('$lib/db.js', () => ({
    getLoginFromToken: vi.fn(),
    getUserMatch: vi.fn(),
    getMatchResults: vi.fn(),
}));

function mockLocation(host = 'localhost:3000') {
    let href = `http://${host}/`;
    const reload = vi.fn();
    const hrefSetter = vi.fn((value) => {
        href = value;
    });
    const fake = {
        protocol: 'http:',
        host,
        reload,
    };
    Object.defineProperty(fake, 'href', {
        get: () => href,
        set: hrefSetter,
        configurable: true,
    });
    vi.stubGlobal('location', fake);
    return { hrefSetter, reload };
}

async function startWebSocketServer() {
    const wss = new WebSocketServer({ host: '127.0.0.1', port: 0 });
    await new Promise((resolve, reject) => {
        wss.once('listening', resolve);
        wss.once('error', reject);
    });

    onTestFinished(() => {
        for (const client of wss.clients) client.terminate();
        wss.close();
    });

    return { wss, port: wss.address().port };
}

describe('home page', () => {
    beforeEach(() => {
        FakeConnectingWebSocket.resetInstances();
        vi.clearAllMocks();
        mocks.page.set({ data: null });
        mocks.nextResult = undefined;
        enhance.mockImplementation(enhanceMock(() => mocks.nextResult));
        vi.stubGlobal('WebSocket', WebSocket);
        db.getLoginFromToken.mockResolvedValue(EXAMPLE_LOGIN);
        db.getUserMatch.mockResolvedValue(null);
        db.getMatchResults.mockResolvedValue([]);
    });

    afterEach(() => {
        cleanup();
        vi.unstubAllGlobals();
    });

    async function renderHome({
        loggedIn = false,
        matches = [],
        match = null,
        host,
    } = {}) {
        const cookies = createCookies();
        if (loggedIn) cookies.set('session', EXAMPLE_SESSION_TOKEN);
        db.getMatchResults.mockResolvedValue(matches);
        db.getUserMatch.mockResolvedValue(match);

        const data = {
            ...((await layout.load({ cookies })) ?? {}),
            ...((await pageLoad.load({ cookies })) ?? {}),
        };
        mocks.page.set({ data });

        const location = mockLocation(host);
        const result = render(Page);
        return { ...result, location, cookies };
    }

    describe('auth', () => {
        it('shows "Log in" and no play button when logged out', async () => {
            await renderHome({ loggedIn: false });

            expect(screen.getByTestId('title')).toHaveTextContent('Log in');
            expect(screen.queryByTestId('play')).not.toBeInTheDocument();
        });

        it('shows the login name and a Play button when logged in', async () => {
            await renderHome({ loggedIn: true });

            expect(screen.getByTestId('title')).toHaveTextContent(
                EXAMPLE_LOGIN
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Play');
        });

        it('logs out and invalidates the page data', async () => {
            await renderHome({ loggedIn: true });
            mocks.nextResult = { type: 'success', data: {} };

            await fireEvent.submit(screen.getByTestId('logout-form'));

            await waitFor(() => expect(invalidateAll).toHaveBeenCalled());
        });

        it('shows the logged out page when the session is removed from the database', async () => {
            const cookies = createCookies();
            cookies.set('session', EXAMPLE_SESSION_TOKEN);
            db.getLoginFromToken.mockResolvedValue(null);

            const data = await layout.load({ cookies });

            expect(data).toBeNull();
            expect(cookies.get('session')).toBeUndefined();

            mocks.page.set({ data });
            mockLocation();
            render(Page);

            expect(screen.getByTestId('title')).toHaveTextContent('Log in');
            expect(screen.queryByTestId('play')).not.toBeInTheDocument();
        });

        it('navigates to the login and register pages', async () => {
            await renderHome({ loggedIn: false });
            const user = userEvent.setup();

            await user.click(screen.getByTestId('login-page'));
            expect(goto).toHaveBeenCalledWith('/login');

            await user.click(screen.getByText('Register'));
            expect(goto).toHaveBeenCalledWith('/register');
        });
    });

    describe('match results', () => {
        it('renders the results table with players and winners', async () => {
            await renderHome({
                loggedIn: true,
                matches: [
                    {
                        details: {
                            players: ['login', 'user2'],
                            winner: 'login',
                        },
                        canceled: false,
                    },
                    {
                        details: {
                            players: ['user3', 'user4'],
                            winner: 'user4',
                        },
                        canceled: false,
                    },
                ],
            });

            expect(getTableMatrix(screen.getByRole('table'))).toEqual([
                ['Player 1', 'Player 2', 'Winner'],
                ['login', 'user2', 'login'],
                ['user3', 'user4', 'user4'],
            ]);
        });

        it('filters out matches without details', async () => {
            await renderHome({
                loggedIn: true,
                matches: [
                    { details: null, canceled: true },
                    {
                        details: {
                            players: ['login', 'user2'],
                            winner: 'login',
                        },
                        canceled: false,
                    },
                ],
            });

            expect(getTableMatrix(screen.getByRole('table'))).toEqual([
                ['Player 1', 'Player 2', 'Winner'],
                ['login', 'user2', 'login'],
            ]);
        });

        it('renders an empty table when there are no matches', async () => {
            await renderHome({ loggedIn: true, matches: [] });

            expect(getTableMatrix(screen.getByRole('table'))).toEqual([[]]);
        });
    });

    describe('matchmaking', () => {
        it('joins an existing match without opening a websocket', async () => {
            const { location } = await renderHome({
                loggedIn: true,
                match: EXAMPLE_MATCH,
            });
            const user = userEvent.setup();

            expect(screen.getByTestId('play')).toHaveTextContent('Join');

            await user.click(screen.getByTestId('play'));

            expect(location.hrefSetter).toHaveBeenCalledWith(
                'http://game/h/p/T'
            );
            expect(screen.queryByTestId('error')).not.toBeInTheDocument();
        });

        it('connects to the server and shows a cancel timer when matchmaking starts', async () => {
            const { wss, port } = await startWebSocketServer();
            await renderHome({ loggedIn: true, host: `127.0.0.1:${port}` });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));

            const [serverSocket] = await connection;
            expect(serverSocket.readyState).toBe(WebSocket.OPEN);
            expect(screen.getByTestId('play')).toHaveTextContent(
                'Cancel 00:00'
            );
        });

        it('launches the game and shows Join when the server sends a match', async () => {
            const { wss, port } = await startWebSocketServer();
            const { location } = await renderHome({
                loggedIn: true,
                host: `127.0.0.1:${port}`,
            });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));
            const [serverSocket] = await connection;

            serverSocket.send(
                JSON.stringify({ login: EXAMPLE_LOGIN, ...EXAMPLE_MATCH })
            );
            serverSocket.close(1000, 'match found');

            await waitFor(() =>
                expect(location.hrefSetter).toHaveBeenCalledWith(
                    'http://game/h/p/T'
                )
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Join');
            expect(screen.queryByTestId('error')).not.toBeInTheDocument();
        });

        it('cancels matchmaking when Play is pressed while queued', async () => {
            const { wss, port } = await startWebSocketServer();
            await renderHome({ loggedIn: true, host: `127.0.0.1:${port}` });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));
            const [serverSocket] = await connection;

            await user.click(screen.getByTestId('play'));
            await waitFor(() =>
                expect(serverSocket.readyState).toBe(WebSocket.CLOSED)
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Play');
            expect(screen.queryByTestId('error')).not.toBeInTheDocument();
        });

        it('closes the connection when matchmaking is cancelled while connecting', async () => {
            vi.stubGlobal('WebSocket', FakeConnectingWebSocket);
            await renderHome({ loggedIn: true });
            const user = userEvent.setup();

            await user.click(screen.getByTestId('play'));
            const socket = FakeConnectingWebSocket.instances[0];
            expect(screen.getByTestId('play')).toHaveTextContent(
                'Cancel 00:00'
            );

            await user.click(screen.getByTestId('play'));
            expect(screen.getByTestId('play')).toHaveTextContent('Play');

            socket.onopen();

            expect(socket.closed).toBe(true);
            expect(screen.queryByTestId('error')).not.toBeInTheDocument();
        });

        it('reloads the page when the server closes with code 4000', async () => {
            const { wss, port } = await startWebSocketServer();
            const { location } = await renderHome({
                loggedIn: true,
                host: `127.0.0.1:${port}`,
            });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));
            const [serverSocket] = await connection;

            serverSocket.close(4000, 'Already in match');

            await waitFor(() =>
                expect(location.reload).toHaveBeenCalledTimes(1)
            );
            expect(screen.queryByTestId('error')).not.toBeInTheDocument();
        });

        it('shows an error when the server closes unexpectedly', async () => {
            const { wss, port } = await startWebSocketServer();
            await renderHome({ loggedIn: true, host: `127.0.0.1:${port}` });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));
            const [serverSocket] = await connection;

            serverSocket.close(1001, 'gone');

            await waitFor(() =>
                expect(screen.getByTestId('error')).toHaveTextContent(
                    'Connection issue, try again later'
                )
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Play');
        });

        it('shows an error when the server rejects the session', async () => {
            const { wss, port } = await startWebSocketServer();
            await renderHome({ loggedIn: true, host: `127.0.0.1:${port}` });
            const user = userEvent.setup();

            const connection = once(wss, 'connection');
            await user.click(screen.getByTestId('play'));
            const [serverSocket] = await connection;

            serverSocket.close(4401, 'Invalid session token');

            await waitFor(() =>
                expect(screen.getByTestId('error')).toHaveTextContent(
                    'Connection issue, try again later'
                )
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Play');
        });

        it('shows an error when the connection fails', async () => {
            const port = await getPort();
            await renderHome({ loggedIn: true, host: `127.0.0.1:${port}` });
            const user = userEvent.setup();

            await user.click(screen.getByTestId('play'));

            await waitFor(() =>
                expect(screen.getByTestId('error')).toHaveTextContent(
                    'Connection issue, try again later'
                )
            );
            expect(screen.getByTestId('play')).toHaveTextContent('Play');
        });
    });
});
