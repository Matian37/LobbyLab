import {
    isWaiting,
    addToWaiting,
    deleteFromWaiting,
    getLoginFromToken,
} from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ success: false });
    const response = await getLoginFromToken(token);
    if (response.length == 0) {
        console.debug('session with given token does not exist');
        return json({ success: false });
    }
    const login = response[0].login;

    if (!(await addToWaiting(login))) {
        console.debug(
            'user not added to waiting list, probably already there'
        );
        return json({ success: false });
    }
    return json({ success: true });
}

export async function DELETE({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ success: false });
    const response = await getLoginFromToken(token);
    if (response.length == 0) {
        console.debug('session with given token does not exist');
        return json({ success: false });
    }
    const login = response[0].login;
    await deleteFromWaiting(login);
    return json({ success: true });
}

export async function GET({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ success: false });
    const response = await getLoginFromToken(token);
    if (response.length == 0) return json({ success: false });
    const login = response[0].login;

    if (await isWaiting(login)) return json({ success: true });

    return json({ success: false });
}
