import { json } from '@sveltejs/kit';
import { sessionExist } from '$lib/db.js';
import { validateSession } from '$lib/validate.js';

export async function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;
    return json({ exists: await sessionExist(result.data.token) });
}
