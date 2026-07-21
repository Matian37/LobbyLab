import { findWaitingByLogin, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { handleError } from '$lib/error_handler.js';

export async function GET({cookies}){
    const token = cookies.get('token');
    if(token == undefined)
        return json({sukces: false});
    const response = await getLoginFromToken(token);
    if(response.length == 0)
        return json({sukces: false});
    const login = response[0].login;
    
    if((await findWaitingByLogin(login)).length > 0)
        return json({sukces: true});

    return json({sukces: false});
}