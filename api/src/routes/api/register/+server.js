import { addUser, addSession } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({request, cookies})
{
    const {login, password} = await request.json();
    const result = await addUser(login, password);
    if(!result){
        return json({
            sukces: false,
            msg: "Podany login jest zajęty"
        });
    }
    else{
        const token = await addSession(login);
        cookies.set('token', token, {
            path: '/',
            httpOnly: true,
            secure: true,
            sameSite: 'strict'
        });
        return json({
            sukces: true,
            msg: null
        });
    }
}

