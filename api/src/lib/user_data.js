import {browser} from '$app/environment'

export function updateData(data){
    if(browser)
        localStorage.setItem('logged', JSON.stringify(data));
}

export async function getData(){
    if(browser)
    {
        const dane = JSON.parse(localStorage.getItem('logged'));
        if(!dane || dane == undefined || dane == null) return false;
        const odp = await fetch(`/api/login?token=${dane.token}`);
        const wynik = await odp.json();
        if(wynik.sukces)
            return dane;
        resetData();
        return false;
    }
    return null;
}

export function resetData(){
    if(browser)
        localStorage.clear();
    return null;
}