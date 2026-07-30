import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { ERRORS } from '$lib/errors.js';
import {
    PASSWORD_MIN_LENGTH,
    PASSWORD_MAX_LENGTH,
    LOGIN_MIN_LENGTH,
    LOGIN_MAX_LENGTH,
} from '$lib/constants.js';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();

    if (typeof login !== 'string' || typeof password !== 'string') {
        return ERRORS.invalidCredentialTypes();
    }

    if (login.length < LOGIN_MIN_LENGTH || login.length > LOGIN_MAX_LENGTH) {
        return ERRORS.invalidLoginLength();
    }

    if (
        password.length < PASSWORD_MIN_LENGTH ||
        password.length > PASSWORD_MAX_LENGTH
    ) {
        return ERRORS.invalidPasswordLength();
    }

    if (!(await addUser(login, password))) {
        return ERRORS.loginTaken();
    }

    const token = await addSession(login);
    cookies.set('session', token, {
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'strict',
    });

    return json({});
}
