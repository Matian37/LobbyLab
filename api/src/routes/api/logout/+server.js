import { json } from '@sveltejs/kit';
import { deleteSession } from '$lib/db.js';

export async function POST({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    await deleteSession(token);
    cookies.delete('session', { path: '/' });
    return json({ success: true });
}
