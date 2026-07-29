import { json } from '@sveltejs/kit';
import { verifyPassword, addSession } from '$lib/db.js';
import { PASSWORD_MIN_LENGTH, PASSWORD_MAX_LENGTH, LOGIN_MIN_LENGTH, LOGIN_MAX_LENGTH } from '$lib/constants.js';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (typeof login !== 'string' || typeof password !== 'string') {
        return json({
            success: false,
            msg: 'Login and password must be strings',
        });
    }

    if (
        login.length < LOGIN_MIN_LENGTH ||
        login.length > LOGIN_MAX_LENGTH ||
        password.length < PASSWORD_MIN_LENGTH ||
        password.length > PASSWORD_MAX_LENGTH
    ) {
        return json({
            success: false,
            msg: 'Invalid login or password',
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
