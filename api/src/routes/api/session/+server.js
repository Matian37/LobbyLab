/**
 * GET `/api/session` — reports the login of the session carried by the
 * `session` cookie. When the cookie is well-formed but the session no longer
 * exists, the stale cookie is deleted and `{ "login": null }` is returned.
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
import { json } from '@sveltejs/kit';
import { getLoginFromToken } from '$lib/db.js';
import { validateSession } from '$lib/validate.js';

export async function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    const login = await getLoginFromToken(result.data.token);
    if (login === null) {
        cookies.delete('session', { path: '/' });
        return json({ login: null });
    }
    return json({ login });
}
