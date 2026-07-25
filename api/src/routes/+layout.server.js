import { getLoginFromToken } from '$lib/db';

export async function load ({cookies}){
    const token = cookies.get('token');
    if(token === undefined)
        return null;
    const dbLogin = await getLoginFromToken(token);
    if(dbLogin.length == 0) 
    {
        console.debug("nie istnieje sesja z danym tokenem");
        cookies.delete('token', { path: '/' });
        return null;
    }
    const login = dbLogin[0].login;

    return {login: login};
}