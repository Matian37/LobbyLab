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

        start() {
            this.seconds = 0;
            this.#timer = setInterval(() => {
                this.seconds++;
            }, 1000);
        }

        stop() {
            if (this.#timer === null) return;
            clearInterval(this.#timer);
            this.#timer = null;
        }
    }

    class Matchmaking {
        timer = new Timer();
        errorMessage = $state('');
        status = $state(WaitingStatus.NOT_ACTIVE);
        socket = null;

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

        #cancel() {
            this.status = WaitingStatus.NOT_ACTIVE;
            this.close();
        }

        #join() {
            const { host, port, matchAuthToken } = this.currentMatch;
            let url = env.PUBLIC_GAME_LAUNCH_URL;
            location.href = url
                .replaceAll('{host}', encodeURIComponent(host))
                .replaceAll('{port}', encodeURIComponent(port))
                .replaceAll('{token}', encodeURIComponent(matchAuthToken));
        }

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

<button onclick={() => goto(resolve('/login'))} data-testid="login-page">
    Login
</button>
<button onclick={() => goto(resolve('/register'))}> Register </button>
<form
    method="POST"
    action="?/logout"
    use:enhance={enhanceLogout}
    data-testid="logout-form"
>
    <button type="submit" data-testid="logout"> Log out </button>
</form>
{#if user}
    <button onclick={() => matchmaking.pressButton()} data-testid="play">
        {matchmaking.buttonText}
    </button>
{/if}

<h1 data-testid="title">{title}</h1>

{#if matchmaking.errorMessage}
    <p class="error" data-testid="error">{matchmaking.errorMessage}</p>
{/if}

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

<style>
    .error {
        color: red;
    }
</style>
