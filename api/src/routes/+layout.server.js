import { getLoginFromToken } from '$lib/db';

export async function load({ cookies }) {
    const token = cookies.get('token');
    if (token === undefined) return null;

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        cookies.delete('token', { path: '/' });
        return null;
    }
    return { login: login };
}
