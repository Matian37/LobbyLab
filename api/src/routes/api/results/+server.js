/**
 * GET `/api/results` — returns the authenticated user's historical match
 * results as `{ "matches": [...] }`, most recent first.
 *
 * @param {import('./$types.js').RequestEvent} event
 * @returns {Promise<import('@sveltejs/kit').Response>}
 */
import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { ERRORS } from '$lib/errors.js';
import { validateSession } from '$lib/validate.js';

export async function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    const login = await getLoginFromToken(result.data.token);
    if (login === null) return ERRORS.invalidSessionToken();

    return json({ matches: await getMatchResults(login) });
}
