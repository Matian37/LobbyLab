<script>
    import { enhance } from '$app/forms';
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';
    import Icon from '$lib/components/Icon.svelte';

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

<header class="topbar">
    <a class="brand" href={resolve('/')} aria-label="Home">
        <span class="brand-name">LobbyLab</span>
    </a>

    <nav class="menu">
        <button class="btn" onclick={() => goto(resolve('/'))}>
            <Icon name="back" />
            Back
        </button>
    </nav>
</header>

<div class="auth">
    <div class="card auth-card">
        <h1 class="auth-title">Register</h1>

        <form
            method="POST"
            use:enhance={enhanceRegister}
            class="auth-form"
            data-testid="register-form"
        >
            <label class="field">
                <span class="field-label">Login</span>
                <span class="input-wrap">
                    <Icon name="user" />
                    <input
                        class="input"
                        name="login"
                        data-testid="login-input"
                        autocomplete="username"
                    />
                </span>
            </label>

            <label class="field">
                <span class="field-label">Password</span>
                <span class="input-wrap">
                    <Icon name="lock" />
                    <input
                        class="input"
                        name="password"
                        type="password"
                        data-testid="password-input"
                        autocomplete="new-password"
                    />
                </span>
            </label>

            <button
                type="submit"
                class="btn btn-primary btn-block"
                data-testid="register-apply"
            >
                Register
            </button>
        </form>

        <p
            class="alert auth-alert"
            class:is-hidden={!errorText}
            data-testid="error-text"
            role="alert"
        >
            {#if errorText}
                <Icon name="alert" />
            {/if}
            {errorText}
        </p>
    </div>
</div>
