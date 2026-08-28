<script>
    import { enhance } from '$app/forms';
    import { goto, invalidateAll } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { onMount, onDestroy } from 'svelte';
    import { page } from '$app/stores';
    import { env } from '$env/dynamic/public';
    import { formatButton, matchStatusString } from '$lib/format.js';
    import { WaitingStatus } from '$lib/constants.js';

    class Timer {
        #timer = null;
        seconds = $state(0);

        /**
         * Starts counting seconds from zero.
         */
        start() {
            this.seconds = 0;
            this.#timer = setInterval(() => {
                this.seconds++;
            }, 1000);
        }

        /**
         * Stops counting. Not allowed if already stopped.
         */
        stop() {
            if (this.#timer === null) return;
            clearInterval(this.#timer);
            this.#timer = null;
        }
    }

    /**
     * Client-side matchmaking controller. Drives the home-page button through
     * three states (`NOT_ACTIVE` → `PENDING` → `FOUND`), manages the
     * `/api/connection` websocket while queued, and launches the game client
     * once a match is assigned by substituting the `{host}`, `{port}`, and
     * `{token}` placeholders in `PUBLIC_GAME_LAUNCH_URL`.
     */
    class Matchmaking {
        timer = new Timer();
        errorMessage = $state('');
        status = $state(WaitingStatus.NOT_ACTIVE);
        socket = null;

        /**
         * @param {object|null} currentMatch The user's existing active match,
         *     if any, which transitions straight to `FOUND`.
         */
        constructor(currentMatch) {
            this.currentMatch = currentMatch;

            if (this.currentMatch) {
                this.status = WaitingStatus.FOUND;
            } else {
                this.status = WaitingStatus.NOT_ACTIVE;
            }

            this.buttonText = $derived(
                formatButton(matchmaking.status, matchmaking.timer.seconds)
            );
        }

        /**
         * Routes a button press to the action for the current status
         */
        pressButton() {
            switch (this.status) {
                case WaitingStatus.NOT_ACTIVE:
                    this.#start();
                    break;
                case WaitingStatus.PENDING:
                    this.#cancel();
                    break;
                case WaitingStatus.FOUND:
                    this.#join();
                    break;
            }
        }

        /**
         * Opens the matchmaking websocket and starts the queued timer.
         */
        #start() {
            this.status = WaitingStatus.PENDING;
            this.errorMessage = '';
            this.timer.start();

            const wsProtocol = location.protocol === 'https:' ? 'wss' : 'ws';
            this.socket = new WebSocket(
                `${wsProtocol}://${location.host}/api/connection`
            );

            this.socket.onmessage = (event) => {
                if (this.status !== WaitingStatus.PENDING) return;

                const payload = JSON.parse(event.data);
                this.status = WaitingStatus.FOUND;
                this.currentMatch = payload;

                this.#join();
            };

            this.socket.onclose = (event) => {
                if (this.status !== WaitingStatus.PENDING) return;

                this.status = WaitingStatus.NOT_ACTIVE;

                // if is 'already in match' error code
                if (event.code === 4000) {
                    this.close();
                    location.reload();
                } else {
                    this.errorMessage = 'Connection issue, try again later';
                    this.close();
                }
            };

            this.socket.onerror = () => {
                if (this.status !== WaitingStatus.PENDING) return;

                this.status = WaitingStatus.NOT_ACTIVE;
                this.errorMessage = 'Connection issue, try again later';
                this.close();
            };
        }

        /**
         * Cancels matchmaking by closing the websocket.
         */
        #cancel() {
            this.status = WaitingStatus.NOT_ACTIVE;
            this.close();
        }

        /**
         * Redirects the browser to the filled-in `PUBLIC_GAME_LAUNCH_URL` to
         * launch the game client for the current match.
         */
        #join() {
            const { host, port, matchAuthToken } = this.currentMatch;
            let url = env.PUBLIC_GAME_LAUNCH_URL;
            location.href = url
                .replaceAll('{host}', encodeURIComponent(host))
                .replaceAll('{port}', encodeURIComponent(port))
                .replaceAll('{token}', encodeURIComponent(matchAuthToken));
        }

        /**
         * Stops the timer and closes the websocket if one is open or still
         * connecting.
         */
        close() {
            this.timer.stop();

            if (!this.socket) return;

            if (this.socket.readyState === WebSocket.CONNECTING) {
                const socket = this.socket;
                socket.onopen = () => socket.close();
            } else {
                this.socket.close();
            }

            this.socket = null;
        }
    }

    let user = $derived($page.data?.login);
    let title = $derived(user ?? 'Log in');
    let matches = $derived($page.data?.matches ?? []);
    let currentMatch = $page.data?.currentMatch ?? null;

    let matchmaking = new Matchmaking(currentMatch);

    const enhanceLogout = () => {
        return async ({ result }) => {
            if (result.type === 'success') {
                await invalidateAll();
            }
        };
    };

    onMount(() => {
        invalidateAll();
    });

    onDestroy(() => {
        matchmaking.close();
    });
</script>

{#if !user}
    <button onclick={() => goto(resolve('/login'))} data-testid="login-page">
        Login
    </button>
    <button
        onclick={() => goto(resolve('/register'))}
        data-testid="register-page"
    >
        Register
    </button>
{:else}
    <form
        method="POST"
        action="?/logout"
        use:enhance={enhanceLogout}
        data-testid="logout-form"
    >
        <button type="submit" data-testid="logout"> Log out </button>
    </form>
    <button onclick={() => matchmaking.pressButton()} data-testid="play">
        {matchmaking.buttonText}
    </button>
    <button
        onclick={() => (location.href = resolve('/api/download'))}
        data-testid="download-client"
    >
        Download client
    </button>
{/if}

{#if matchmaking.errorMessage}
    <p class="error" data-testid="error">{matchmaking.errorMessage}</p>
{/if}

<h1 data-testid="title">{title}</h1>

{#if user}
    <h1 data-testid="matches-title">Matches</h1>
    <table>
        <thead>
            <tr>
                <th>ID</th>
                <th>STATUS</th>
                <th>PLAYERS</th>
                <th>WINNER</th>
            </tr>
        </thead>
        <tbody>
            {#if matches.length > 0}
                {#each matches as match (match.id)}
                    <tr>
                        <td>{match.id}</td>
                        <td>{matchStatusString(match)}</td>
                        <td>{match.details?.players?.join(', ') ?? ''}</td>
                        <td>{match.details?.winner ?? ''}</td>
                    </tr>
                {/each}
            {/if}
        </tbody>
    </table>
{/if}
