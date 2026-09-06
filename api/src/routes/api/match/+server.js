/**
 * GET `/api/match` — returns the active match assigned to the authenticated
 * user, if any, as `{ "match": {...} | null }`.
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
import { getUserMatch, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { ERRORS } from '$lib/errors.js';
import { validateSession } from '$lib/validate.js';

export async function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    const login = await getLoginFromToken(result.data.token);
    if (login === null) return ERRORS.invalidSessionToken();

    return json({ match: await getUserMatch(login) });
}
