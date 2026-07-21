import { getMatchResults, getLoginFromToken } from '$lib/db.js';
import { handleError } from '$lib/error_handler.js';
import { json } from '@sveltejs/kit';

export async function GET({cookies}){
    const token = cookies.get('token');
    if(token == undefined)
        return json({sukces: false, matches: null});
    const dbLogin = await getLoginFromToken(token);
    if(dbLogin.length == 0) 
    {
        handleError(0);
        return json({sukces: false, matches: null});
    }
    const login = dbLogin[0].login;
    //matches -> array of objects (.details[.players{array of str}, .winner{str}], .canceled[bool])
    const matches = await getMatchResults(login)

    //const matches = await getMatchResults('adam');
    return json({sukces: true, matches: matches})
}