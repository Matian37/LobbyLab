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
            if(await isInWaitingList(user.login)) buttonText = 'Stop';
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
        user = getData();
        if(!user) 
        {
            console.debug('zaloguj sie~!!');
            return;
        }
        let meth = 'POST';
        if(await isInWaitingList(user.login)) meth = 'DELETE';

        const response = await fetch('/api/waiting', {
            method: meth,
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({login: user.login})
        });

        const wynik = await response.json();
        if(wynik.sukces){ 
            if(meth == 'POST')
            {
                buttonText = 'Stop';

                stream = new EventSource(`/api/connection?login=${user.login}`);
            }
            else buttonText = 'Play';
        }
        else
            console.debug("cos poszlo nie tak z dobieraniem graczy");
    }

    async function isInWaitingList(login)
    {
        let response = await fetch(`/api/waiting?login=${login}`);
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