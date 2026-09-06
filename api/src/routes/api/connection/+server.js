/**
 * GET `/api/connection` — deliberately rejects a plain HTTP request. The real
 * WebSocket handshake is handled outside of SvelteKit by the connection server
 * attached to the HTTP server; here any plain GET is answered with `426
 * WebSocket Required`.
 */
import { ERRORS } from '$lib/errors';

export function GET() {
    return ERRORS.wsRequired();
}
