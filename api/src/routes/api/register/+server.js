import { addUser } from '$lib/db.js';
import bcrypt from 'bcryptjs';
import { json } from '@sveltejs/kit';
import { generateToken } from '$lib/user_data.js';

export async function POST({request})
{
    const {login, password} = await request.json();
    const hashed = await bcrypt.hash(password, 10);
    const result = addUser(login, hashed);
    let sukces = true;
    if(!result) sukces = false;

    return json({
        sukces: sukces,
        token: generateToken(login)
    });
}