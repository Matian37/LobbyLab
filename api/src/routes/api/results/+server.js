import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function GET({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ sukces: false, matches: null });

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('nie istnieje sesja z danym tokenem');
        return json({ sukces: false, matches: null });
    }
    return json({ sukces: true, matches: await getMatchResults(login) });
}
