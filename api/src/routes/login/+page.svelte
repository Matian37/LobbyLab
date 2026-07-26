<script>
    import { goto } from '$app/navigation';

    let login = $state(''),
        password = $state(''),
        errorMessage = $state('');

    function changePage(path) {
        goto(path);
    }

    async function submit() {
        let data = { login: login, password: password };
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(data),
        });
        console.log('response received');

        const success = await response.json();
        if (success.success) {
            console.log('logged in successfully');
            errorMessage = '';
            changePage('/');
        } else {
            errorMessage = success.msg;
        }
    }
</script>

<button onclick={() => changePage('/')}>Back</button>
Login <input bind:value={login} data-testid="login-input" />
Password
<input bind:value={password} type="password" data-testid="password-input" />
<button onclick={() => submit()} data-testid="login-apply">Submit</button>

<p data-testid="error-text">{errorMessage}</p>

<style>
</style>
