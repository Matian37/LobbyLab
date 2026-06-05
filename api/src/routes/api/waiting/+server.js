import { findWaitingByLogin, addToWaiting, deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { handleError } from '$lib/error_handler.js';

export async function POST({request}){   
    const {token} = await request.json();
    const login = getLoginFromToken(token);
    if(!login) 
    {
        handleError(0);
        return json({sukces: false});
    }
        
    if(!addToWaiting(login))
    {
        handleError(1);
        return json({sukces: false});
    }
    return json({sukces: true});
}

export async function DELETE({request}){
    const {token} = await request.json();
    const login = getLoginFromToken(token);
    if(!login) 
    {
        handleError(0);
        return json({sukces: false});
    }
        
    if(!deleteFromWaiting(login)) 
    {
        handleError(2);
        return json({sukces: false});
    }
    return json({sukces: true});
}

export async function GET({url}){
    const token = url.searchParams.get('token');
    const login = getLoginFromToken(token);
    if(!login)
    {
        handleError(0);
        return json({sukces: false});
    }
    
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}