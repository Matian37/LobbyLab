import { GET } from '$routes/api/session/+server.js';

export async function load({ cookies }) {
    const response = await GET({ cookies });
    if (!response.ok) return null;

    const { login } = await response.json();
    if (login === null) return null;
    return { login };
}
