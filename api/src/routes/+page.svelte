<script>
    import { goto } from "$app/navigation";
    import { getData, resetData } from "$lib/user_data";
    let title = $state('Zaloguj sie');
    let buttonText = $state('Play');
    let user = false;
    let rows = $state([]);

    LoadUser();
    async function LoadUser(){
        let data = await getData();
        if(!data || data == undefined) title = "Zaloguj sie";
        else{
            user = {login: data.login, token: data.token};
            title = data.login;
            LoadMatches(data.token);
        } 
    }

    async function LoadMatches(token){
        const query = await fetch(`/api/results?token=${token}`);
        const response = await query.json();
        if(!response.sukces)
            return;
        rows = response.matches;
    }

    export function changePage(path) {
        console.log("KLIKKKK");
        goto(path);
    }

    async function logout(){
        if(!user) return;
        resetData();
        const response = await fetch('/api/login', {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({token: user.token})
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
        if(await isInWaitingList(user.token)){
            console.debug('jestes juz w kolejce');
            return;
        }
    }

    async function isInWaitingList(token)
    {
        let response = await fetch(`/api/waiting?token=${token}`);
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