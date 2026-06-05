import { addUser, setSession, tokenExists } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';

export async function POST({request})
{
    const {login, password} = await request.json();
    const hashed = await bcrypt.hash(password, 10);
    const result = addUser(login, hashed);
    if(!result){
        return json({
            sukces: false,
            msg: "Podany login jest zajęty"
        });
    }
    else{
        return json({
            sukces: true,
            msg: generateToken(login)
        });
    }
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