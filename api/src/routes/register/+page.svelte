<script>
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';

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
        const result = await response.json();
        if (!result.success) {
            errorText = result.msg;
        } else {
            console.debug('registered successfully');
            await goto(resolve('/'));
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
