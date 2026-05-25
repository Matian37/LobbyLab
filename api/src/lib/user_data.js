import {browser} from '$app/environment'

export function updateData(data){
    if(browser)
        localStorage.setItem('logged', JSON.stringify(data));
}

export function getData(){
    if(browser)
        return JSON.parse(localStorage.getItem('logged'));
    return null;
}

export function resetData(){
    if(browser)
        localStorage.clear();
}
