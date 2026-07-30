import { getLoginFromToken, extendQueueStatus, getAuthToken } from '$lib/db.js';
import { json } from '@sveltejs/kit';
import postgres from 'postgres';
import { ERRORS } from '$lib/errors.js';

export async function GET({ cookies }) {
    const token = cookies.get('session');
    if (token == undefined) return ERRORS.noSessionToken();

    const login = await getLoginFromToken(token);
    if (login === null) {
        console.debug('session with given token does not exist');
        return ERRORS.invalidSessionToken();
    }

    let interval, dbInterval;
    return new Response(
        new ReadableStream({
            async start(controller) {
                await listen(login, controller);
                interval = setInterval(() => {
                    console.debug('sending ping');
                    controller.enqueue('data: ping\n\n');
                }, 10000);

                dbInterval = setInterval(() => {
                    // TODO: on false, stop connection
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

    await sql.listen('users_match_id_assigned', async (payload) => {
        const parsed = JSON.parse(payload);
        if (parsed.username != login) return;

        parsed.match_auth_token = await getAuthToken(login);

        console.debug('sending server socket');
        controller.enqueue(`data: ${JSON.stringify(parsed)}\n\n`);
        controller.close();
    });
}
