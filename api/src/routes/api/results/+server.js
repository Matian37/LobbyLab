import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { handleError } from '$lib/error_handler.js';
import { json } from '@sveltejs/kit';

export async function GET({url}){
    const token = url.searchParams.get('token')
    const dbLogin = await getLoginFromToken(token);
    if(dbLogin.length == 0) 
    {
        handleError(0);
        return json({sukces: false, matches: null});
    }
    const login = dbLogin[0].login;
    const matches = await getMatchResults(login)
    return json({sukces: true, matches: matches})
}