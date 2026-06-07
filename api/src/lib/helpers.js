import { setSession, tokenExists } from "./db.js";

export function generateToken(login){
    let result = '';
    const len = 16;
    do
    {
        for(let i = 0; i < len; i++)
            result += String.fromCharCode(Math.floor(Math.random() * 43) + 48);
    }
    while(tokenExists(result));
    setSession(result, login);
    return result;
}