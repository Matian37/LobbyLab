<script>
    import { enhance } from '$app/forms';
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';

    let errorText = $state('');

    const enhanceRegister = () => {
        return async ({ result }) => {
            if (result.type === 'failure') {
                errorText = result.data?.msg ?? UNEXPECTED_ERROR_MSG;
                return;
            }
            if (result.type === 'error') {
                errorText = UNEXPECTED_ERROR_MSG;
                return;
            }
            errorText = '';
            await goto(resolve('/'));
        };
    };
</script>

<button onclick={() => goto(resolve('/'))}>Back</button>
<form method="POST" use:enhance={enhanceRegister} data-testid="register-form">
    Login <input name="login" data-testid="login-input" />
    Password
    <input name="password" type="password" data-testid="password-input" />
    <button type="submit" data-testid="register-apply"> Submit </button>
</form>
<p data-testid="error-text">{errorText}</p>


<style>

</style>