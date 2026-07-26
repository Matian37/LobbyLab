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

    if (await verifyPassword(login, password)) {
        const token = await addSession(login);
        cookies.set('token', token, {
            path: '/',
            httpOnly: true,
            secure: true,
            sameSite: 'strict',
        });
        return json({
            success: true,
            msg: null,
        });
    } else {
        return json({
            success: false,
            msg: 'Invalid login or password',
        });
    }
}

export async function DELETE({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ success: false });

    await deleteSession(token);
    cookies.delete('token', { path: '/' });
    return json({ success: true });
}

export async function GET({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ success: false });

    return json({ success: await tokenExists(token) });
}
