import { json } from '@sveltejs/kit';
import { deleteSession } from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';

export async function POST({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return ERRORS.noSessionToken();

    await deleteSession(token);
    cookies.delete('session', { path: '/' });
    return json({});
}
