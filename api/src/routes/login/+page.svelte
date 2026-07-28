<script>
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';

    let login = $state(''),
        password = $state(''),
        errorMessage = $state('');

    async function submit() {
        const response = await fetch('/api/login', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ login: login, password: password }),
        });
        console.log('response received');

        const success = await response.json();
        if (success.success) {
            console.log('logged in successfully');
            errorMessage = '';
            await goto(resolve('/'));
        } else {
            errorMessage = success.msg;
        }
    }
</script>

<button onclick={() => goto(resolve('/'))}>Back</button>
Login <input bind:value={login} data-testid="login-input" />
Password
<input bind:value={password} type="password" data-testid="password-input" />
<button onclick={() => submit()} data-testid="login-apply">Submit</button>

<p data-testid="error-text">{errorMessage}</p>

<style>
</style>
