import {
    verifyPassword,
    addSession,
    deleteSession,
    getLoginFromToken,
    tokenExists,
} from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (!(await verifyPassword(login, password))) {
        return json({
            success: false,
            msg: 'Invalid login or password',
        });
    }

    const token = await addSession(login);
    cookies.set('session', token, {
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'strict',
    });

    return json({
        success: true,
        msg: null,
    });
}

export async function DELETE({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    await deleteSession(token);
    cookies.delete('session', { path: '/' });
    return json({ success: true });
}

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });
    return json({ success: await tokenExists(token) });
}
