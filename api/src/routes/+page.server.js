/**
 * Home page server logic. The loader returns the authenticated user's match
 * results and any active match, and the `logout` action delegates to
 * `POST /api/logout`.
 *
 * @param {import('./$types.js').PageServerLoadEvent} event
 * @returns {Promise<{ matches: Array, currentMatch: object|null }>}
 */
import { fail } from '@sveltejs/kit';
import { getUserMatch, getMatchResults, getLoginFromToken } from '$lib/db.js';
import { validateSession } from '$lib/validate.js';
import { POST as logoutPost } from '$routes/api/logout/+server.js';

export async function load({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) {
        return { matches: [], currentMatch: null };
    }

    const login = await getLoginFromToken(result.data.token);
    if (login === null) {
        return { matches: [], currentMatch: null };
    }

    return {
        matches: await getMatchResults(login),
        currentMatch: await getUserMatch(login),
    };
}

/**
 * Form actions for the home page. Currently only `logout`, which delegates to
 * `POST /api/logout` and surfaces its response as a form failure when needed.
 */
export const actions = {
    logout: async ({ cookies }) => {
        const response = await logoutPost({ cookies });
        if (response.ok) return {};
        return fail(response.status, await response.json());
    },
};
