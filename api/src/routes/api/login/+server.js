import { findUserByLogin, setSession, tokenExists, deleteSession, getLoginFromToken } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';

export async function POST({request})
{
    const {login, password} = await request.json();
    const result = findUserByLogin(login);
    if(!result) {
        return json({
        sukces: false
        });
    }
    
    if(await bcrypt.compare(password, result.password)){
        return json({
            sukces: true,
            token: generateToken(login)
        })
    }
    else{
        return json({
            sukces: false,
            token: null
        })
    }
}

export async function DELETE({request}){
    const {token} = await request.json();

    if(!deleteSession(token)) return json({sukces: false});
    return json({sukces: true});
}

function generateToken(login){
    let result = '';
    const len = 16;
    do
    {
        for(let i = 0; i < len; i++)
            result += String.fromCharCode(Math.floor(Math.random() * 43) + 48);
    }
    while(tokenExists(result));
    setSession(result, login);
    return result;
}