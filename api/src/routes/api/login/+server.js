import { findUserByLogin, addSession, deleteSession, getLoginFromToken, tokenExists } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';

export async function POST({request, cookies})
{
    const {login, password} = await request.json();
    const result = await findUserByLogin(login);
    if(result.length == 0) {
        return json({sukces: false, msg: "Podany login nie istnieje"});
    }
    
    if(await bcrypt.compare(password, result[0].password)){
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
    else{
        return json({
            sukces: false,
            msg: "Podane hasło jest błędne"
        });
    }
}

export async function DELETE({cookies}){
    const token = cookies.get('token');
    if(token == undefined)
        return json({sukces: false});

    await deleteSession(token);
    cookies.delete('token', { path: '/' });
    return json({sukces: true});
}

export async function GET({cookies}){
    const token = cookies.get('token');
    if(token == undefined)
        return json({sukces: false});
    
    if((await tokenExists(token)).length == 0) return json({sukces: false});
    return json({sukces: true});
}