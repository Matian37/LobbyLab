<script>
    import { goto } from "$app/navigation";
    import { onMount } from "svelte";
    import { page } from '$app/stores';
    import { invalidateAll } from '$app/navigation';
    let buttonText = $state('Play');
    let rows = $state([]);
    let user = $derived($page.data);
    let title = $derived(!user.login ? 'Zaloguj sie' : user.login);
    
    onMount(async () => {
        invalidateAll();
        LoadMatches();
    });

    async function LoadMatches(){
        if(!user)
            return;
        const query = await fetch(`/api/results`, {
            method: 'GET'
        });
        const response = await query.json();
        if(!response.sukces)
            return;
        rows = response.matches;
    }

    export function changePage(path) {
        goto(path);
    }

    async function logout(){
        if(!user) return;
        const response = await fetch('/api/login', {
            method: 'DELETE'
        });
        user = null;

        title = "Zaloguj sie";
    }

    async function play(){
        if(!user) 
        {
            console.debug('zaloguj sie~!!');
            return;
        }
        if(await isInWaitingList()){
            console.debug('jestes juz w kolejce');
            return;
        }
        
    }

    async function isInWaitingList()
    {
        let response = await fetch(`/api/waiting`, {
            method: 'GET',
        }
        );
        let wynik = await response.json();
        return wynik.sukces;
    }
</script>

<button onclick={() => changePage("/login")} data-testid="login-page">
    Login
</button>
<button onclick={() => changePage("/register")}>
    Register
</button>
<button onclick={() => logout()} data-testid='logout'>
    Log out
</button>
<button onclick={()=> play()}>
    {buttonText}
</button>

<h1 data-testid='title'>{title}</h1>

<table>
    <thead>
      <tr>
        {#if rows.length > 0}
            {#each Array(rows.players.length) as _, i}
                <th>Player {i + 1}</th>
            {/each}
            <th>Winner</th>
        {/if}
      </tr>
    </thead>
    <tbody>
        {#if rows.length > 0}
            {#each rows as row}
                <tr>
                    {#each row.players as player}
                        <td>{player}</td>
                    {/each}
                    <td>{row.winner}</td>
                </tr>
            {/each}
        {/if}
    </tbody>
  </table>