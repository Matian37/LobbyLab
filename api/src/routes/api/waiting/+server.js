import { findWaitingByLogin, addToWaiting, deleteFromWaiting } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function POST({request}){   
    try{
        const {login} = await request.json();
        const res = addToWaiting(login);
        return json({sukces: true});
    }
    catch{
        return json({sukces: false});
    }
}

export async function DELETE({request}){
    try{
        const {login} = await request.json();
        const res = deleteFromWaiting(login);
        return json({sukces: true});
    }
    catch{
        return json({sukces: false});
    }
}

export async function GET({url}){
    const login = url.searchParams.get('login');
    if(findWaitingByLogin(login))
        return json({sukces: true});

    return json({sukces: false});
}