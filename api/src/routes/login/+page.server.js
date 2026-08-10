import { fail } from '@sveltejs/kit';
import { POST as loginPOST } from '$routes/api/login/+server.js';

export const actions = {
    default: async ({ request, cookies }) => {
        const formData = await request.formData();
        const response = await loginPOST({
            request: {
                json: async () => ({
                    login: formData.get('login'),
                    password: formData.get('password'),
                }),
            },
            cookies,
        });
        if (response.ok) return {};
        return fail(response.status, await response.json());
    },
};
