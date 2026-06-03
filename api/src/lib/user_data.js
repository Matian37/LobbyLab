import {browser} from '$app/environment'
import { tokenExists, setSession } from '$lib/db.js'

export function updateData(data){
    if(browser)
        localStorage.setItem('logged', JSON.stringify(data));
}

export function getData(){
    if(browser)
        return JSON.parse(localStorage.getItem('logged'));
}

export function resetData(){
    if(browser)
        localStorage.clear();
}

export function generateToken(login){
    let result = '';
    const len = 16;
    do
    {
        for(let i = 0; i < len; i++)
            result += String.fromCharCode(Mathf.floor(Math.random() * 43) + 48);
    }
    while(tokenExists(result));
    setSession(result, login);
    return result;
}