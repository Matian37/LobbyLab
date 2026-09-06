<script>
    import '../app.css';

    let { children } = $props();
</script>

<div class="shell">
    <main class="main">
        {@render children()}
    </main>

    <footer class="footer">Powered by LobbyLab</footer>
</div>
