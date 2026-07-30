import { json } from '@sveltejs/kit';
import { sessionExist } from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return ERRORS.noSessionToken();
    return json({ exists: await sessionExist(token) });
}
