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
