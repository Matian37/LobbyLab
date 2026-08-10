<script>
    import { enhance } from '$app/forms';
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';

    let errorMessage = $state('');

    const enhanceLogin = () => {
        return async ({ result }) => {
            if (result.type === 'failure') {
                errorMessage = result.data?.msg ?? UNEXPECTED_ERROR_MSG;
                return;
            }
            if (result.type === 'error') {
                errorMessage = UNEXPECTED_ERROR_MSG;
                return;
            }
            errorMessage = '';
            await goto(resolve('/'));
        };
    };
</script>

<button onclick={() => goto(resolve('/'))}>Back</button>
<form method="POST" use:enhance={enhanceLogin} data-testid="login-form">
    Login <input name="login" data-testid="login-input" />
    Password
    <input name="password" type="password" data-testid="password-input" />
    <button type="submit" data-testid="login-apply">Submit</button>
</form>

<p data-testid="error-text">{errorMessage}</p>

<style>
</style>
