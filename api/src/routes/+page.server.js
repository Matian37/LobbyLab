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

export const actions = {
    logout: async ({ cookies }) => {
        const response = await logoutPost({ cookies });
        if (response.ok) return {};
        return fail(response.status, await response.json());
    },
};
