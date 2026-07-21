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
        console.log(response.matches);
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
                {#each Array(rows[0].details.players.length) as _, i}
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
                    {#each row.details.players as player}
                        <td>{player}</td>
                    {/each}
                    <td>{row.details.winner}</td>
                </tr>
            {/each}
        {/if}
    </tbody>
</table>
<style>
    /* Wyśrodkowanie przycisków */
    button {
      display: inline-block;
      margin: 0.5rem 0.25rem;
      padding: 0.6rem 1.2rem;
      font-size: 0.95rem;
      font-weight: 500;
      color: #f8fafc;
      background-color: #1e293b;
      border: 1px solid #334155;
      border-radius: 8px;
      cursor: pointer;
      transition: all 0.2s ease;
    }
  
    button:hover {
      background-color: #334155;
      border-color: #475569;
    }
  
    /* Styl dla ostatniego przycisku (Play) */
    button:last-of-type {
      background-color: #2563eb;
      border-color: #3b82f6;
      font-weight: 600;
    }
  
    button:last-of-type:hover {
      background-color: #1d4ed8;
    }
  
    /* Wyśrodkowanie całości tekstu i nagłówka */
    :global(body) {
      text-align: center;
      font-family: system-ui, -apple-system, sans-serif;
      background-color: #0f172a;
      color: #f8fafc;
      padding: 2rem;
    }
  
    h1 {
      margin: 1.5rem 0;
      color: #38bdf8;
    }
  
    /* Wyśrodkowanie tabeli na stronie */
    table {
      margin: 1.5rem auto;
      border-collapse: collapse;
      background-color: #1e293b;
      border-radius: 8px;
      overflow: hidden;
      box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.3);
    }
  
    th, td {
      padding: 0.75rem 1.25rem;
      text-align: center;
      border-bottom: 1px solid #334155;
    }
  
    th {
      background-color: #334155;
      color: #94a3b8;
      font-size: 0.85rem;
      text-transform: uppercase;
    }
  
    td:last-child {
      color: #4ade80;
      font-weight: 600;
    }
</style>