<script>
    import { goto } from '$app/navigation';

    let login = $state(''),
        password = $state(''),
        tekstBledu = $state('');

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
        console.log('wrocilo mi info');

        const sukces = await response.json();
        if (sukces.sukces) {
            console.log('poprawnie zalogowano');
            tekstBledu = '';
            changePage('/');
        } else {
            tekstBledu = sukces.msg;
        }
    }
</script>

<button onclick={() => changePage('/')}>Powrót</button>
Login <input bind:value={login} data-testid="login-input" />
Password
<input bind:value={password} type="password" data-testid="password-input" />
<button onclick={() => submit()} data-testid="login-apply">Submit</button>

<p data-testid="error-text">{tekstBledu}</p>

<style>
</style>
