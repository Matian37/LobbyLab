import { findUserByLogin } from '$lib/db.js';
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
            user: result
        })
    }
    else{
        return json({
            sukces: false
        })
    }
}