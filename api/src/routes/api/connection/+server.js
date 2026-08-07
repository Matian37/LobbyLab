import { validateSession } from '$lib/validate.js';

// TODO: just return response; don't validate session cookie

// The connection endpoint is served over WebSocket, which is set up outside of
// SvelteKit (see src/lib/server/server.js). A plain HTTP GET cannot be upgraded,
// so it is rejected.
export function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    return new Response('WebSocket connection required', { status: 426 });
}
