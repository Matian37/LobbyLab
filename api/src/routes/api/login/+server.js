import { findUserByLogin, setSession, deleteSession, getLoginFromToken, tokenExists } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';
import { handleError } from '$lib/error_handler.js';
import { generateToken } from '$lib/helpers.js';

export async function POST({request})
{
    const {login, password} = await request.json();
    const result = await findUserByLogin(login);
    if(result.length == 0) {
        return json({sukces: false, msg: "Podany login nie istnieje"});
    }
    
    if(await bcrypt.compare(password, result[0].password)){
        return json({
            sukces: true,
            msg: generateToken(login)
        });
    }
    else{
        return json({
            sukces: false,
            msg: "Podane hasło jest błędne"
        });
    }
}

export async function DELETE({request}){
    const {token} = await request.json();

    await deleteSession(token);
    return json({sukces: true});
}

export async function GET({url}){
    const token = url.searchParams.get('token');
    
    if((await tokenExists(token)).length == 0) return json({sukces: false});
    return json({sukces: true});
}