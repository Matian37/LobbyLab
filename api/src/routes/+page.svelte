<script>
    import { enhance } from '$app/forms';
    import { goto, invalidateAll } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { onMount, onDestroy } from 'svelte';
    import { page } from '$app/stores';
    import { env } from '$env/dynamic/public';
    import { formatButton, matchStatusString } from '$lib/format.js';
    import { WaitingStatus } from '$lib/constants.js';
    import Icon from '$lib/components/Icon.svelte';

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
        }

        /**
         * Label for the matchmaking button, derived from the live status and the
         * queue timer.
         */
        get buttonText() {
            return formatButton(this.status, this.timer.seconds);
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
    let matchmakingStatusClass = $derived(
        user && matchmaking.status === WaitingStatus.PENDING
            ? 'is-pending'
            : user && matchmaking.status === WaitingStatus.FOUND
              ? 'is-found'
              : ''
    );

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

<header class="topbar">
    <a class="brand" href={resolve('/')} aria-label="Home">
        <span class="brand-name {matchmakingStatusClass}">LobbyLab</span>
    </a>

    <nav class="menu">
        {#if !user}
            <button
                class="btn"
                onclick={() => goto(resolve('/login'))}
                data-testid="login-page"
            >
                <Icon name="login" />
                Log in
            </button>
            <button
                class="btn btn-primary"
                onclick={() => goto(resolve('/register'))}
                data-testid="register-page"
            >
                <Icon name="register" />
                Register
            </button>
        {:else}
            <button
                class="btn"
                onclick={() => (location.href = resolve('/api/download'))}
                data-testid="download-client"
            >
                <Icon name="download" />
                Download client
            </button>
            <form
                method="POST"
                action="?/logout"
                use:enhance={enhanceLogout}
                data-testid="logout-form"
            >
                <button type="submit" class="btn" data-testid="logout">
                    <Icon name="logout" />
                    Log out
                </button>
            </form>
        {/if}
    </nav>
</header>

<section class="hero">
    <p class="eyebrow {matchmakingStatusClass}">Competitive matchmaking</p>
    <h1 class="title" data-testid="title">{title}</h1>

    {#if !user}
        <p class="subtitle">
            Log in or create an account to start the matchmaking.
        </p>
    {:else}
        <button
            class="btn btn-primary btn-lg btn-play {matchmakingStatusClass}"
            onclick={() => matchmaking.pressButton()}
            data-testid="play"
        >
            {#if matchmaking.status === WaitingStatus.NOT_ACTIVE}
                <Icon name="play" />
            {/if}
            {matchmaking.buttonText}
        </button>

        {#if matchmaking.status === WaitingStatus.PENDING}
            <p class="queue-hint" aria-live="polite">
                <span class="pulse" aria-hidden="true"></span>
                Searching for opponents…
            </p>
        {/if}
    {/if}

    {#if matchmaking.errorMessage}
        <p class="alert" data-testid="error" role="alert">
            <Icon name="alert" />
            {matchmaking.errorMessage}
        </p>
    {/if}
</section>

{#if user}
    <div class="card results">
        <h2 class="results-title" data-testid="matches-title">Matches</h2>
        <div class="table-wrap">
            <table class="table">
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
                                <td class="cell-id">{match.id}</td>
                                <td>
                                    <span
                                        class="status status-{matchStatusString(
                                            match
                                        ).toLowerCase()}"
                                        >{matchStatusString(match)}</span
                                    >
                                </td>
                                <td
                                    >{match.details?.players?.join(', ') ??
                                        ''}</td
                                >
                                <td class="cell-winner"
                                    >{match.details?.winner ?? ''}</td
                                >
                            </tr>
                        {/each}
                    {/if}
                </tbody>
            </table>
        </div>
    </div>
{/if}
