import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as layout from './+layout.server.js';
import { GET } from '$routes/api/session/+server.js';
import { createCookies } from '$lib/test/utils.js';

vi.mock('$routes/api/session/+server.js', () => ({
    GET: vi.fn(),
}));

function jsonResponse(body, status = 200) {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    });
}

describe('load', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns null when there is no session cookie', async () => {
        GET.mockResolvedValue(
            jsonResponse({ msg: 'No session token provided' }, 401)
        );
        const cookies = createCookies();

        const result = await layout.load({ cookies });

        expect(result).toBeNull();
        expect(GET).toHaveBeenCalledWith({ cookies });
    });

    it('returns the login when the session is valid', async () => {
        GET.mockResolvedValue(jsonResponse({ login: 'login' }));
        const cookies = createCookies();

        const result = await layout.load({ cookies });

        expect(result).toEqual({ login: 'login' });
        expect(GET).toHaveBeenCalledWith({ cookies });
    });

    it('returns null when the session does not exist', async () => {
        GET.mockResolvedValue(jsonResponse({ login: null }));
        const cookies = createCookies();

        const result = await layout.load({ cookies });

        expect(result).toBeNull();
        expect(GET).toHaveBeenCalledWith({ cookies });
    });

    it('propagates handler failures', async () => {
        GET.mockRejectedValue(new Error('database is down'));
        const cookies = createCookies();

        await expect(layout.load({ cookies })).rejects.toThrow(
            'database is down'
        );
    });
});
