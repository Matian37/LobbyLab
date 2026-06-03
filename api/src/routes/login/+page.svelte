<script>
    import { goto } from "$app/navigation";
    import { updateData } from "$lib/user_data.js";

    let login = $state(''), password = $state(''), pokazBlad = $state(false);
    
    function changePage(path){
        goto(path);
    }

    async function submit(){
        let data = {login: login, password: password};
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        });

        const sukces = await response.json();
        if(sukces.sukces){
            updateData({token: sukces.token, login: login});
            pokazBlad = false;
            changePage('/')
        }
        else{
            pokazBlad = true;
        }
    }
</script>

<button onclick={()=>changePage('/')}>Powrót</button>
Login <input bind:value={login}/>
Password <input bind:value={password} type="password"/>
<button onclick={()=>submit()}>Submit</button>
{#if pokazBlad}
Blad logowania
{/if}

<style>

</style>