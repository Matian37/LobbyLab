import { isWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { ERRORS } from '$lib/errors.js';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return ERRORS.noSessionToken();

    const login = await getLoginFromToken(token);
    if (login === null) return ERRORS.invalidSessionToken();

    return json({ waiting: await isWaiting(login) });
}
