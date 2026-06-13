<script>
    import { goto } from "$app/navigation";
    import { updateData } from "$lib/user_data";

    let login = $state(''), password = $state(''), errorText = $state('');
    function changePage(path){
        goto(path);
    }

    async function submit(){
        if(password.length < 3 || password.length > 64)
        {
            errorText = 'Haslo musi miec co najmniej 3 znaki i co najwyżej 64';
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
Login <input bind:value={login} data-testid="login-input"/>
Password <input bind:value={password} type="password" data-testid="password-input"/>
<button onclick={()=>submit()} data-testid='register-apply'> Submit </button>
<p data-testid="error-text">{errorText}</p>
<style>

</style>