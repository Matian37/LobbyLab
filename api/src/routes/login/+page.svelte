<script>
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';

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

        if (response.ok) {
            console.log('logged in successfully');
            errorMessage = '';
            await goto(resolve('/'));
        } else {
            const err = await response
                .json()
                .catch(() => ({ msg: UNEXPECTED_ERROR_MSG }));
            errorMessage = err.msg;
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
