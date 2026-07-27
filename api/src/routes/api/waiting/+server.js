import {
    isWaiting,
    addToWaiting,
    deleteFromWaiting,
    getLoginFromToken,
} from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        return json({ success: false });
    }

    if (!(await addToWaiting(login))) {
        console.debug('user not added to waiting list, probably already there');
        return json({ success: false });
    }
    return json({ success: true });
}

export async function DELETE({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        return json({ success: false });
    }

    if (await deleteFromWaiting(login)) {
        console.debug('user removed from waiting list');
        return json({ success: true });
    }
    return json({ success: false });
}

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    const login = await getLoginFromToken(token);
    if (login === null) return json({ success: false });

    return json({ success: await isWaiting(login) });
}
