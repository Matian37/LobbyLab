import { getLoginFromToken } from '$lib/db';
import { handleError } from '$lib/error_handler';

export async function load ({cookies}){
    const token = cookies.get('token');
    if(token === undefined)
        return null;
    const dbLogin = await getLoginFromToken(token);
    if(dbLogin.length == 0) 
    {
        handleError(0);
        cookies.delete('token', { path: '/' });
        return null;
    }
    const login = dbLogin[0].login;

    return {login: login};
}