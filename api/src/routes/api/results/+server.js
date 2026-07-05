import { getMatchResults } from '$lib/db.js';
import { handleError } from '$lib/error_handler.js';
import { json } from 'node:stream/consumers';

export async function GET({url}){
    const login = url.searchParams.get('login')
    const matches = getMatchResults(login)

    return json(matches)
}