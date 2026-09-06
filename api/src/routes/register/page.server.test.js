import { vi, it, expect, describe, beforeEach } from 'vitest';
import { actions } from './+page.server.js';
import { POST } from '$routes/api/register/+server.js';

vi.mock('$routes/api/register/+server.js', () => ({
    POST: vi.fn(),
}));

function formRequest(login, password) {
    const formData = new FormData();
    formData.set('login', login);
    formData.set('password', password);
    return { formData: async () => formData };
}

function jsonResponse(body, status = 200) {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    });
}

describe('register action', () => {
    beforeEach(() => {
        vi.clearAllMocks();
    });

    it('returns empty json when the handler responds ok', async () => {
        POST.mockResolvedValue(jsonResponse({}));

        const result = await actions.default({
            request: formRequest('login', 'password'),
            cookies: {},
        });

        expect(POST).toHaveBeenCalledTimes(1);
        expect(await POST.mock.calls[0][0].request.json()).toEqual({
            login: 'login',
            password: 'password',
        });
        expect(result).toEqual({});
    });

    it('returns fail with the status and body when the handler responds not ok', async () => {
        POST.mockResolvedValue(
            jsonResponse({ msg: 'Login is already taken' }, 409)
        );

        const result = await actions.default({
            request: formRequest('login', 'password'),
            cookies: {},
        });

        expect(POST).toHaveBeenCalledTimes(1);
        expect(await POST.mock.calls[0][0].request.json()).toEqual({
            login: 'login',
            password: 'password',
        });
        expect(result).toMatchObject({
            status: 409,
            data: { msg: 'Login is already taken' },
        });
    });
});
