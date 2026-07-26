import { deleteFromWaiting, getLoginFromToken } from '$lib/db.js';
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

    let interval;
    return new Response(
        new ReadableStream({
            start(controller) {
                //listen(login, controller);
                interval = setInterval(() => {
                    console.debug('sending ping');
                    controller.enqueue('data: ping\n\n');
                }, 10000);
            },
            async cancel() {
                clearInterval(interval);
                console.debug('client disconnected SSE connection');
                await deleteFromWaiting(login);
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
        if (payload.username != login) return;
        console.debug('sending server socket');
        controller.equeue(`data: ${payload}\n\n`);
        controller.close();
    });
}
