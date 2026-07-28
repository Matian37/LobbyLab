import { json } from '@sveltejs/kit';
import { sessionExist } from '$lib/db.js';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });
    return json({ success: await sessionExist(token) });
}
