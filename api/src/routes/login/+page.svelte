<script>
    import { enhance } from '$app/forms';
    import { goto } from '$app/navigation';
    import { resolve } from '$app/paths';
    import { UNEXPECTED_ERROR_MSG } from '$lib/errors.js';
    import Icon from '$lib/components/Icon.svelte';

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
        <h1 class="auth-title">Log in</h1>

        <form
            method="POST"
            use:enhance={enhanceLogin}
            class="auth-form"
            data-testid="login-form"
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
                        autocomplete="current-password"
                    />
                </span>
            </label>

            <button
                type="submit"
                class="btn btn-primary btn-block"
                data-testid="login-apply"
            >
                Log in
            </button>
        </form>

        <p
            class="alert auth-alert"
            class:is-hidden={!errorMessage}
            data-testid="error-text"
            role="alert"
        >
            {#if errorMessage}
                <Icon name="alert" />
            {/if}
            {errorMessage}
        </p>
    </div>
</div>
