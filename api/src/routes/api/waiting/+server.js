import { findWaitingByLogin, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    const login = await getLoginFromToken(token);
    if (login === null) return json({ success: false });

    return json({ success: (await findWaitingByLogin(login)).length > 0 });
}
