import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { ERRORS } from '$lib/errors.js';
import { validateCredentialsSchema } from '$lib/validate.js';

export async function POST({ request, cookies }) {
    const result = await validateCredentialsSchema(request);
    if (result.error !== undefined) return result.error;

    if (!(await addUser(result.data.login, result.data.password))) {
        return ERRORS.loginTaken();
    }

    const token = await addSession(result.data.login);
    cookies.set('session', token, {
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'strict',
    });

    return json({});
}
