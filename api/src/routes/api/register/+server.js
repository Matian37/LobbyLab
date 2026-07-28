import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (password.length < 5 || password.length > 64) {
        return json({
            success: false,
            msg: 'Password must be between 5 and 64 characters',
        });
    }

    if (!(await addUser(login, password))) {
        return json({
            success: false,
            msg: 'Login is already taken',
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
