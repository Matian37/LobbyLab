import { addUser, setSession } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';
import { generateToken } from '$lib/helpers.js';

export async function POST({request, cookies})
{
    const {login, password} = await request.json();
    const hashed = await bcrypt.hash(password, 10);
    const result = await addUser(login, hashed);
    if(!result){
        return json({
            sukces: false,
            msg: "Podany login jest zajęty"
        });
    }
    else{
        const token = await generateToken(login);
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

