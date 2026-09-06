<script>
    import {
        ArrowLeft,
        CircleAlert,
        CircleUser,
        Download,
        Lock,
        LogIn,
        LogOut,
        Play,
        UserPlus,
    } from 'lucide-svelte';

    /**
     * Thin wrapper around `lucide-svelte` that maps a semantic icon name to a
     * Lucide component, so pages reference icons by intent rather than SVG paths.
     *
     * @param {'login'|'register'|'download'|'logout'|'back'|'user'|'lock'|'play'|'alert'} name
     */
    let { name, ...rest } = $props();

    const icons = {
        login: LogIn,
        register: UserPlus,
        download: Download,
        logout: LogOut,
        back: ArrowLeft,
        user: CircleUser,
        lock: Lock,
        play: Play,
        alert: CircleAlert,
    };

    const IconComponent = $derived(icons[name]);
</script>

<IconComponent class="icon" {...rest} />
