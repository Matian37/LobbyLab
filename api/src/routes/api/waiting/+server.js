import { findWaitingByLogin, addToWaiting, deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { handleError } from '$lib/error_handler.js';

export async function POST({request}){   
    const {token} = await request.json();
    const response = await getLoginFromToken(token);
    if(response.length == 0) 
    {
        handleError(0);
        return json({sukces: false});
    }
    const login = response[0].login;
        
    if(!addToWaiting(login))
    {
        handleError(1);
        return json({sukces: false});
    }
    return json({sukces: true});
}

export async function DELETE({request}){
    const {token} = await request.json();
    const response = await getLoginFromToken(token);
    if(response.length == 0) 
    {
        handleError(0);
        return json({sukces: false});
    }
    const login = response[0].login;
    if(!deleteFromWaiting(login)) 
    {
        handleError(2);
        return json({sukces: false});
    }
    return json({sukces: true});
}

export async function GET({url}){
    const token = url.searchParams.get('token');
    const response = await getLoginFromToken(token);
    if(response.length == 0)
        return json({sukces: false});
    const login = response[0].login;
    
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}