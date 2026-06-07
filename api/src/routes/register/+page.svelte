<script>
    import { goto } from "$app/navigation";
    import { updateData } from "$lib/user_data";

    let login = $state(''), password = $state(''), errorText = $state('');
    function changePage(path){
        goto(path);
    }

    async function submit(){
        if(password.length < 3)
        {
            errorText = 'Haslo musi miec co najmniej 3 znaki';
            return;
        }
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
            errorText = result.msg;
        }
        else{
            console.debug("poprawnie zarejestrowano");
            updateData({token: result.msg, login: login});
            changePage('/');
        }
    }
</script>

<button onclick={()=>changePage('/')}>Powrót</button>
Login <input bind:value={login}/>
Password <input bind:value={password} type="password"/>
<button onclick={()=>submit()}> Submit </button>
{errorText}
<style>

</style>