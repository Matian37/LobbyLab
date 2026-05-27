import { deleteFromWaiting } from '$lib/db.js';
import { json } from '@sveltejs/kit';

export async function GET({url}){
    const login = url.searchParams.get('login');
    let interval;
    return new Response(
        new ReadableStream({
            start(controller){
                interval = setInterval(()=>{
                    controller.enqueue('data: ping\n\n');
                }, 2000);
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