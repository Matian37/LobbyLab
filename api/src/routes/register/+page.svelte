<script>
    import { goto } from "$app/navigation";

    let login = $state(''), password = $state(''), error = $state(false);
    function changePage(path){
        goto(path);
    }

    async function submit(){
        let data = {login: login, password: password};
        let response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify(data)
        });
        const result = await response.json();
        if(!result.sukces){
            error = true;
        }
        else{
            changePage('/');
        }
    }
</script>

<button onclick={()=>changePage('/')}>Powrót</button>
Login <input bind:value={login}/>
Password <input bind:value={password} type="password"/>
<button onclick={()=>submit()}> Submit </button>
{#if error}
Blad logowania
{/if}
<style>

</style>