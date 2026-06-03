import { findWaitingByLogin, addToWaiting, deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({request}){   
    try{
        const {token} = await request.json();
        const login = getLoginFromToken(token);
        
        const res = addToWaiting(login);
        return json({sukces: true});
    }
    catch{
        return json({sukces: false});
    }
}

export async function DELETE({request}){
    try{
        const {token} = await request.json();
        const login = getLoginFromToken(token);
        
        const res = deleteFromWaiting(login);
        return json({sukces: true});
    }
    catch{
        return json({sukces: false});
    }
}

export async function GET({url}){
    const token = url.searchParams.get('token');
    const login = getLoginFromToken(token);
    
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}