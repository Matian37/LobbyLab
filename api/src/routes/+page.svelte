<script>
    import { goto } from "$app/navigation";
    import { getData, resetData } from "$lib/user_data";
    let title = $state('');

    const user = getData();
    if(!user) title = "Zaloguj sie";
    else title = user.login;

    function changePage(path) {
        goto(path);
    }

    function logout(){
        resetData();
        title = "Zaloguj sie";
    }
</script>

<button onclick={() => changePage("/login")}>
    Login
</button>
<button onclick={() => changePage("/register")}>
    Register
</button>
<button onclick={() => logout()}>
    Log out
</button>

<h1>{title}</h1>

<style>

</style>