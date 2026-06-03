<script>
    import { goto } from "$app/navigation";
    import { getData, resetData } from "$lib/user_data";
    import { onMount } from "svelte";
    let title = $state('');
    let buttonText = $state('Play');

    let user = getData();
    if(!user) title = "Zaloguj sie";
    else{
        title = user.login;
        onMount(async () =>{
            if(await isInWaitingList(user.token)) buttonText = 'Stop';
        });
    }

    function changePage(path) {
        goto(path);
    }

    function logout(){
        resetData();
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
        //window.location.href = `game-run://webplay?login=${user.login}`;
    }

    async function isInWaitingList(token)
    {
        let response = await fetch(`/api/waiting?token=${token}`);
        let wynik = await response.json();
        return wynik.sukces;
    }
</script>

<button onclick={() => changePage("/login")}>
    Login
</button>
<button onclick={() => changePage("/register")}>
    Register
</button>
<button onclick={() => logout()}>
    Log out
</button>
<button onclick={()=> play()}>
    {buttonText}
</button>

<h1>{title}</h1>

<style>

</style>