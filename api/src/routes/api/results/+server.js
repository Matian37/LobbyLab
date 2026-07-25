import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function GET({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ sukces: false, matches: null });
    const dbLogin = await getLoginFromToken(token);
    if (dbLogin.length == 0) {
        console.debug('nie istnieje sesja z danym tokenem');
        return json({ sukces: false, matches: null });
    }
    const login = dbLogin[0].login;
    const matches = await getMatchResults(login);
    return json({ sukces: true, matches: matches });
}
