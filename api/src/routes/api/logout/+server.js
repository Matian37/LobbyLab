/**
 * POST `/api/logout` — deletes the session identified by the `session` cookie
 * and clears the cookie.
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
import { json } from '@sveltejs/kit';
import { deleteSession } from '$lib/db.js';
import { validateSession } from '$lib/validate.js';

export async function POST({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    await deleteSession(result.data.token);

    cookies.delete('session', { path: '/' });
    return json({});
}
