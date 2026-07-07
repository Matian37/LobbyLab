import { findWaitingByLogin, addToWaiting, deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { handleError } from '$lib/error_handler.js';

export async function POST({cookies}){   
    const token = cookies.get('token');
    if(token === undefined)
        return json({sukces: false});
    const response = await getLoginFromToken(token);
    if(response.length == 0) 
    {
        handleError(0);
        return json({sukces: false});
    }
    const login = response[0].login;
    
    if(!(await addToWaiting(login)))
    {
        handleError(1);
        return json({sukces: false});
    }
    return json({sukces: true});
}

export async function DELETE({cookies}){
    const token = cookies.get('token');
    if(token === undefined)
        return json({sukces: false});
    const response = await getLoginFromToken(token);
    if(response.length == 0) 
    {
        handleError(0);
        return json({sukces: false});
    }
    const login = response[0].login;
    await deleteFromWaiting(login);
    return json({sukces: true});
}

export async function GET({cookies}){
    const token = cookies.get('token');
    if(token === undefined)
        return json({sukces: false});
    const response = await getLoginFromToken(token);
    if(response.length == 0)
        return json({sukces: false});
    const login = response[0].login;
    
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}