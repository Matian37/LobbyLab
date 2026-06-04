import { findWaitingByLogin, addToWaiting, deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({request}){   
    const {token} = await request.json();
    const login = getLoginFromToken(token);
    if(!login) return json({sukces: false});
        
    if(!addToWaiting(login)) return json({sukces: false});
    return json({sukces: true});

}

export async function DELETE({request}){
    const {token} = await request.json();
    const login = getLoginFromToken(token);
    if(!login) return json({sukces: false});
        
    if(!deleteFromWaiting(login)) return json({sukces: false});
    return json({sukces: true});
}

export async function GET({url}){
    const token = url.searchParams.get('token');
    const login = getLoginFromToken(token);
    if(!login) return json({sukces: false});
    
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}