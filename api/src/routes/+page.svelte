<script>
    import { enhance } from '$app/forms';
    import { goto, invalidateAll } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { onMount, onDestroy } from 'svelte';
    import { page } from '$app/stores';
    import { env } from '$env/dynamic/public';

    let user = $derived($page.data?.login);
    let title = $derived(user ?? 'Log in');
    let matches = $derived($page.data?.matches ?? []);
    let currentMatch = $state($page.data?.currentMatch ?? null);
    let matchmaking = $state(false);
    let matchmakingError = $state('');
    let matchmakingSeconds = $state(0);
    let matchmakingSocket = null;
    let matchmakingTimer = null;
    let matchFound = false;
    let cancelled = false;

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
        matchmakingSocket?.close();
        stopTimer();
    });

    async function play() {
        if (!user) {
            console.debug('log in first');
            return;
        }

        if (currentMatch) {
            launchGame(currentMatch);
            return;
        }

        if (matchmaking) {
            cancelMatchmaking();
            return;
        }

        matchmakingError = '';
        matchmaking = true;
        cancelled = false;
        startTimer();

        const wsProtocol = location.protocol === 'https:' ? 'wss' : 'ws';
        matchmakingSocket = new WebSocket(
            `${wsProtocol}://${location.host}/api/connection`
        );
        matchmakingSocket.onmessage = (event) => {
            const payload = JSON.parse(event.data);
            if (payload.host !== undefined && payload.port !== undefined) {
                matchFound = true;
                currentMatch = payload;
                launchGame(payload);
            }
        };
        matchmakingSocket.onclose = (event) => {
            matchmaking = false;
            stopTimer();
            if (matchFound) return;
            if (cancelled) return;
            if (event.code === 4000) {
                location.reload();
                return;
            }
            matchmakingError = 'Connection issue, try again later';
        };
        matchmakingSocket.onerror = () => {
            matchmaking = false;
            stopTimer();
            if (matchFound) return;
            if (cancelled) return;
            matchmakingError = 'Connection issue, try again later';
        };
    }

    function cancelMatchmaking() {
        cancelled = true;
        if (matchmakingSocket) {
            if (matchmakingSocket.readyState === WebSocket.OPEN) {
                matchmakingSocket.close();
            } else if (matchmakingSocket.readyState === WebSocket.CONNECTING) {
                const socket = matchmakingSocket;
                socket.onopen = () => socket.close();
            }
        }
        matchmakingSocket = null;
        matchmaking = false;
        stopTimer();
        matchmakingError = '';
    }

    function launchGame(match) {
        const url = env.PUBLIC_GAME_LAUNCH_URL.replaceAll(
            '{host}',
            encodeURIComponent(match.host)
        )
            .replaceAll('{port}', encodeURIComponent(match.port))
            .replaceAll('{token}', encodeURIComponent(match.matchAuthToken));
        location.href = url;
    }

    function matchStatus(match) {
        if (match.active) return 'ACTIVE';
        if (match.canceled) return 'CANCELED';
        return 'FINISHED';
    }

    function startTimer() {
        matchmakingSeconds = 0;
        matchmakingTimer = setInterval(() => {
            matchmakingSeconds++;
        }, 1000);
    }

    function stopTimer() {
        if (matchmakingTimer !== null) {
            clearInterval(matchmakingTimer);
            matchmakingTimer = null;
        }
    }

    function formatTime(totalSeconds) {
        const minutes = String(Math.floor(totalSeconds / 60)).padStart(2, '0');
        const seconds = String(totalSeconds % 60).padStart(2, '0');
        return `${minutes}:${seconds}`;
    }
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
    <button onclick={() => play()} data-testid="play">
        {currentMatch
            ? 'Join'
            : matchmaking
              ? `Cancel ${formatTime(matchmakingSeconds)}`
              : 'Play'}
    </button>
{/if}

<h1 data-testid="title">{title}</h1>

{#if matchmakingError}
    <p class="error" data-testid="error">{matchmakingError}</p>
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
                    <td>{matchStatus(match)}</td>
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
