import { json } from '@sveltejs/kit';
import { verifyPassword, addSession } from '$lib/db.js';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (typeof login !== 'string' || typeof password !== 'string') {
        return json({
            success: false,
            msg: 'Login and password must be strings',
        });
    }

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
