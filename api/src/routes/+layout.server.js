/**
 * Root layout loader. Determines the current session's login by delegating to
 * the `GET /api/session` handler, and returns `{ login }` only when the session
 * is valid.
 *
 * @param {import('./$types.js').LayoutServerLoadEvent} event
 * @returns {Promise<{ login: string } | null>}
 */
import { GET } from '$routes/api/session/+server.js';

export async function load({ cookies }) {
    const response = await GET({ cookies });
    if (!response.ok) return null;

    const { login } = await response.json();
    if (login === null) return null;
    return { login };
}
