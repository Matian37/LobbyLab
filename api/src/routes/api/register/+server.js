import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from '$lib/constants.js';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (typeof login !== 'string' || typeof password !== 'string') {
        return json({
            success: false,
            msg: 'Login and password must be strings',
        });
    }

    if (login.length < LOGIN_MIN_LENGTH || login.length > LOGIN_MAX_LENGTH) {
        return json({
            success: false,
            msg: `Login must be at least ${LOGIN_MIN_LENGTH} and at most ${LOGIN_MAX_LENGTH} characters`,
        });
    }

    if (
        password.length < PASSWORD_MIN_LENGTH ||
        password.length > PASSWORD_MAX_LENGTH
    ) {
        return json({
            success: false,
            msg: `Password must be at least ${PASSWORD_MIN_LENGTH} and at most ${PASSWORD_MAX_LENGTH} characters`,
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
