/**
 * POST `/api/login` — authenticates credentials and starts a session.
 *
 * Body: `{ "login": string, "password": string }`. On success a `session`
 * cookie is set (`httpOnly`, `secure`, `sameSite: strict`). The password used
 * is stored in the session, so a deleted user is reported as invalid
 * credentials.
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
import { json } from '@sveltejs/kit';
import { verifyPassword, addSession } from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { validateCredentialsSchema } from '$lib/validate.js';
import { httpLogger } from '$lib/logger.js';

export async function POST({ request, cookies }) {
    const result = await validateCredentialsSchema(request);
    if (result.error !== undefined) return result.error;

    if (!(await verifyPassword(result.data.login, result.data.password))) {
        return ERRORS.invalidCredentials();
    }

    const token = await addSession(result.data.login);
    if (token === null) {
        // user gone, so credentials are no longer valid from user perspective
        httpLogger.warn(
            { login: result.data.login },
            'login failed, user no longer exists'
        );
        return ERRORS.invalidCredentials();
    }

    cookies.set('session', token, {
        path: '/',
        httpOnly: true,
        secure: true,
        sameSite: 'strict',
    });
    return json({});
}
