import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false, matches: null });

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        return json({ success: false, matches: null });
    }
    return json({ success: true, matches: await getMatchResults(login) });
}
