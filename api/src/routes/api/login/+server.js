import {
    findUserByLogin,
    verifyPassword,
    addSession,
    deleteSession,
    getLoginFromToken,
    tokenExists,
} from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({ request, cookies }) {
    const { login, password } = await request.json();
    const user = await findUserByLogin(login);
    if (user === null) {
        return json({ sukces: false, msg: 'Podany login nie istnieje' });
    }

    if (await verifyPassword(login, password)) {
        const token = await addSession(login);
        cookies.set('token', token, {
            path: '/',
            httpOnly: true,
            secure: true,
            sameSite: 'strict',
        });
        return json({
            sukces: true,
            msg: null,
        });
    } else {
        return json({
            sukces: false,
            msg: 'Podane hasło jest błędne',
        });
    }
}

export async function DELETE({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ sukces: false });

    await deleteSession(token);
    cookies.delete('token', { path: '/' });
    return json({ sukces: true });
}

export async function GET({ cookies }) {
    const token = cookies.get('token');
    if (token == undefined) return json({ sukces: false });

    return json({ suckes: await tokenExists(token) });
}
