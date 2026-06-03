import { findUserByLogin } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';
import { generateToken } from '$lib/user_data.js';

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
            sukces: false
        })
    }
}