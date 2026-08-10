import { fail } from '@sveltejs/kit';
import { POST as registerPOST } from '$routes/api/register/+server.js';

export const actions = {
    default: async ({ request, cookies }) => {
        const formData = await request.formData();
        const response = await registerPOST({
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
