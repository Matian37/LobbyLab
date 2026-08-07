<script>
    import { goto, invalidateAll } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { onMount, onDestroy } from 'svelte';
    import { page } from '$app/stores';
    import { env } from '$env/dynamic/public';

    let rows = $state([]);
    let user = $derived($page.data);
    let title = $derived(!user.login ? 'Log in' : user.login);
    let matchmaking = $state(false);
    let matchmakingError = $state('');
    let matchmakingSeconds = $state(0);
    let currentMatch = $state(null);
    let matchmakingSocket = null;
    let matchmakingTimer = null;
    let matchFound = false;

    onMount(async () => {
        invalidateAll();
        LoadMatches();

        if (!user) return;
        await loadCurrentMatch();
    });

    onDestroy(() => {
        matchmakingSocket?.close();
        stopTimer();
    });

    async function LoadMatches() {
        if (!user) return;
        const query = await fetch(`/api/results`, {
            method: 'GET',
        });
        if (!query.ok) return;

        const response = await query.json();

        rows = response.matches.filter((row) => row.details !== null);
    }

    async function loadCurrentMatch() {
        const response = await fetch('/api/match', {
            method: 'GET',
        });
        if (!response.ok) return;

        const { match } = await response.json();
        currentMatch = match;
    }

    async function logout() {
        if (!user) return;
        await fetch('/api/logout', {
            method: 'POST',
        });
        user = null;

        title = 'Log in';
    }

    async function play() {
        if (!user) {
            console.debug('log in first');
            return;
        }

        if (currentMatch) {
            launchGame(currentMatch);
            return;
        }

        if (matchmaking) return;

        matchmakingError = '';
        matchmaking = true;
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
            matchmakingError = 'Connection issue, try again later';
        };
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
<button onclick={() => logout()} data-testid="logout"> Log out </button>
{#if user.login}
    <button onclick={() => play()} data-testid="play">
        {currentMatch
            ? 'Join'
            : matchmaking
              ? formatTime(matchmakingSeconds)
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
            {#if rows.length > 0}
                {#each Array(rows[0].details.players.length) as _, i (i)}
                    <th>Player {i + 1}</th>
                {/each}
                <th>Winner</th>
            {/if}
        </tr>
    </thead>
    <tbody>
        {#if rows.length > 0}
            {#each rows as row, i (i)}
                <tr>
                    {#each row.details.players as player (player)}
                        <td>{player}</td>
                    {/each}
                    <td>{row.details.winner}</td>
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
