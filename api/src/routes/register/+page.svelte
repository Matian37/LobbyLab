<script>
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';

    let login = $state(''),
        password = $state(''),
        errorText = $state('');

    async function submit() {
        let data = { login: login, password: password };
        let response = await fetch('/api/register', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(data),
        });
        if (response.ok) {
            console.debug('registered successfully');
            await goto(resolve('/'));
        } else {
            const err = await response
                .json()
                .catch(() => ({ msg: UNEXPECTED_ERROR_MSG }));
            errorText = err.msg;
        }
    }
</script>

<button onclick={() => goto(resolve('/'))}>Back</button>
Login <input bind:value={login} data-testid="login-input" />
Password
<input bind:value={password} type="password" data-testid="password-input" />
<button onclick={() => submit()} data-testid="register-apply"> Submit </button>
<p data-testid="error-text">{errorText}</p>

<style>
</style>
