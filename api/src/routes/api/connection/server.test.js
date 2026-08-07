import { it, describe } from 'vitest';
import * as api from './+server.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test/utils.js';

describe('GET', () => {
    it('returns error when session cookie is missing', async () => {
        const response = api.GET();
        await expectError(response, ERRORS.wsRequired);
    });
});
