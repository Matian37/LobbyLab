import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();
    const result = await addUser(login, password);

    if (!result) {
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
