<script>
    import { goto, invalidateAll } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { onMount } from 'svelte';
    import { page } from '$app/stores';

    let buttonText = $state('Play');
    let rows = $state([]);
    let user = $derived($page.data);
    let title = $derived(!user.login ? 'Log in' : user.login);

    onMount(async () => {
        invalidateAll();
        LoadMatches();
    });

    async function LoadMatches() {
        if (!user) return;
        const query = await fetch(`/api/results`, {
            method: 'GET',
        });
        const response = await query.json();

        if (!response.success) return;
        rows = response.matches;
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
        if (await isInWaitingList()) {
            console.debug('already in queue');
            return;
        }
    }

    async function isInWaitingList() {
        let response = await fetch(`/api/waiting`, {
            method: 'GET',
        });
        let wynik = await response.json();
        return wynik.success;
    }
</script>

<button onclick={() => goto(resolve('/login'))} data-testid="login-page">
    Login
</button>
<button onclick={() => goto(resolve('/register'))}> Register </button>
<button onclick={() => logout()} data-testid="logout"> Log out </button>
<button onclick={() => play()}>
    {buttonText}
</button>

<h1 data-testid="title">{title}</h1>

<table>
    <thead>
        <tr>
            {#if rows.length > 0}
                {#each Array(rows.players.length) as _, i (i)}
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
                    {#each row.players as player (player)}
                        <td>{player}</td>
                    {/each}
                    <td>{row.winner}</td>
                </tr>
            {/each}
        {/if}
    </tbody>
</table>
