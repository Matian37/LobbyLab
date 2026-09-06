/**
 * POST `/api/register` — creates a user account and starts a session for them.
 *
 * Body: `{ "login": string, "password": string }`. On success a `session`
 * cookie is set (`httpOnly`, `secure`, `sameSite: strict`).
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
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
