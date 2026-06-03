import { deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import { Client } from 'pg';

export async function GET({url}){
    const token = url.searchParams.get('token');
    const login = getLoginFromToken(token);
    
    let interval;
    return new Response(
        new ReadableStream({
            start(controller){
                listen(login, controller);
                interval = setInterval(()=>{
                    controller.enqueue('data: ping\n\n');
                }, 10000);
            },
            cancel(){
                clearInterval(interval);
                deleteFromWaiting(login);
            }
        }),
        {
            headers: {
                'Content-Type': 'text/event-stream',
                'Cache-Control': 'no-cache',
                'Connection': 'keep-alive'
            }
        }
    );
}

async function listen(login, controller){
    const client = new Client({
        connectionString: process.env.DATABASE_URL,
    });
      
    await client.connect();
      
    await client.query("LISTEN users_match_id_assigned");
      
    client.on("notification", (msg) => {
        if(msg.payload.username != login) return;
        controller.equeue(`data: ${msg.payload}\n\n`);
        controller.close();
    });
}