import { vi, it, expect, describe, beforeEach } from 'vitest';
import * as page from './+page.server.js';
import * as db from '$lib/db.js';
import { POST } from '$routes/api/logout/+server.js';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';
import { createCookies } from '$lib/test/utils.js';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);
const EXAMPLE_MATCH = { host: 'h', port: 'p', matchAuthToken: 't' };

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
        getUserMatch: vi.fn(),
        getMatchResults: vi.fn(),
    };
});

vi.mock('$routes/api/logout/+server.js', () => ({
    POST: vi.fn(),
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

    it('returns empty defaults when there is no session cookie', async () => {
        const cookies = createCookies();

        const result = await page.load({ cookies });

        expect(result).toEqual({ matches: [], currentMatch: null });
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
        expect(db.getMatchResults).not.toHaveBeenCalled();
        expect(db.getUserMatch).not.toHaveBeenCalled();
    });

    it('returns empty defaults when the session does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue(null);
        const cookies = createCookies();
        cookies.set('session', EXAMPLE_SESSION_TOKEN);

        const result = await page.load({ cookies });

        expect(result).toEqual({ matches: [], currentMatch: null });
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
        expect(db.getMatchResults).not.toHaveBeenCalled();
        expect(db.getUserMatch).not.toHaveBeenCalled();
    });

    it('loads the matches and the current match for a valid session', async () => {
        const matches = [
            {
                details: { players: ['login', 'user2'], winner: 'login' },
                canceled: false,
            },
        ];
        db.getLoginFromToken.mockResolvedValue('login');
        db.getMatchResults.mockResolvedValue(matches);
        db.getUserMatch.mockResolvedValue(EXAMPLE_MATCH);
        const cookies = createCookies();
        cookies.set('session', EXAMPLE_SESSION_TOKEN);

        const result = await page.load({ cookies });

        expect(result).toEqual({
            matches,
            currentMatch: EXAMPLE_MATCH,
        });
        expect(db.getMatchResults).toHaveBeenCalledWith('login');
        expect(db.getUserMatch).toHaveBeenCalledWith('login');
    });

    it('propagates database failures', async () => {
        db.getLoginFromToken.mockResolvedValue('login');
        db.getMatchResults.mockRejectedValue(new Error('database is down'));
        const cookies = createCookies();
        cookies.set('session', EXAMPLE_SESSION_TOKEN);

        await expect(page.load({ cookies })).rejects.toThrow(
            'database is down'
        );
    });
});

describe('logout action', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns {} when the handler responds ok', async () => {
        POST.mockResolvedValue(jsonResponse({}));
        const cookies = createCookies();

        const result = await page.actions.logout({ cookies });

        expect(POST).toHaveBeenCalledTimes(1);
        expect(POST.mock.calls[0][0].cookies).toBe(cookies);
        expect(result).toEqual({});
    });

    it('returns fail with the status and body when the handler responds not ok', async () => {
        POST.mockResolvedValue(
            jsonResponse({ msg: 'No session token provided' }, 401)
        );
        const cookies = createCookies();

        const result = await page.actions.logout({ cookies });

        expect(POST).toHaveBeenCalledTimes(1);
        expect(POST.mock.calls[0][0].cookies).toBe(cookies);
        expect(result).toMatchObject({
            status: 401,
            data: { msg: 'No session token provided' },
        });
    });
});
