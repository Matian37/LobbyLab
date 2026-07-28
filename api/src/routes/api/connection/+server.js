import { getLoginFromToken, extendQueueStatus } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { Client } from 'postgres';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return json({ success: false });

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        return json({ success: false });
    }
                    
    let interval, dbInterval;
    return new Response(
        new ReadableStream({
            start(controller){
                await listen(login, controller);
                interval = setInterval(()=>{
                    console.debug("sending ping");
                    controller.enqueue('data: ping\n\n');
                }, 10000);

                dbInterval = setInverval(()=>{
                    extendQueueStatus(login);
                }, 1000);
            },
            async cancel() {
                clearInterval(interval);
                console.debug('client disconnected SSE connection');
                // TODO: set user status to not queued
            },
        }),
        {
            headers: {
                'Content-Type': 'text/event-stream',
                'Cache-Control': 'no-cache',
                Connection: 'keep-alive',
            },
        }
    );
}

async function listen(login, controller) {
    // TODO: route notification instead of spawning connection per client
    const sql = postgres(process.env.DATABASE_URL);

    await sql.listen('users_match_id_assigned', (payload) => {
        // FIX: payload is a string, so this is not valid
        if (payload.username != login) return;

        payload.match_auth_token = await getAuthToken(login);
        
        console.debug('sending server socket');
        controller.equeue(`data: ${payload}\n\n`);
        controller.close();
    });
}
