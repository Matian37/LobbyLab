import { vi, it, expect, describe, beforeEach, afterEach } from 'vitest';
import * as api from './+server.js';
import * as db from '$lib/db.js';
import * as validate from '$lib/validate.js';
import { ERRORS } from '$lib/errors.js';
import { expectError } from '$lib/test/utils.js';
import { SESSION_TOKEN_LENGTH } from '$lib/constants.js';
import { mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';

const EXAMPLE_SESSION_TOKEN = 'a'.repeat(SESSION_TOKEN_LENGTH);

vi.mock('$lib/db.js', () => {
    return {
        getLoginFromToken: vi.fn(),
    };
});

vi.spyOn(validate, 'validateSession');

describe('GET', () => {
    const FILE_NAME = 'game-client.zip';
    const FILE_CONTENT = 'game client bytes';
    let downloadsDir;

    beforeEach(async () => {
        vi.clearAllMocks();
        downloadsDir = await mkdtemp(path.join(os.tmpdir(), 'downloads-'));
        vi.stubEnv('DOWNLOADS_DIR', downloadsDir);
        vi.stubEnv('GAME_CLIENT_FILE', FILE_NAME);
        await writeFile(path.join(downloadsDir, FILE_NAME), FILE_CONTENT);
    });

    afterEach(async () => {
        vi.unstubAllEnvs();
        await rm(downloadsDir, { recursive: true, force: true });
    });

    it('returns error when session cookie is missing', async () => {
        const cookies = { get: () => undefined };
        const response = await api.GET({ cookies });

        await expectError(response, ERRORS.noSessionToken);
        expect(validate.validateSession).toHaveBeenCalledWith(cookies);
        expect(db.getLoginFromToken).not.toHaveBeenCalled();
    });

    it('returns error when the session does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue(null);
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };

        const response = await api.GET({ cookies });

        await expectError(response, ERRORS.invalidSessionToken);
        expect(db.getLoginFromToken).toHaveBeenCalledWith(
            EXAMPLE_SESSION_TOKEN
        );
    });

    it('streams the client file with attachment headers', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(200);
        expect(response.headers.get('Content-Type')).toBe(
            'application/octet-stream'
        );
        expect(response.headers.get('Content-Disposition')).toBe(
            `attachment; filename="${FILE_NAME}"`
        );
        expect(response.headers.get('Content-Length')).toBe(
            String(Buffer.byteLength(FILE_CONTENT))
        );
        expect(await response.text()).toBe(FILE_CONTENT);
    });

    it('returns 404 when the file does not exist', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        vi.stubEnv('GAME_CLIENT_FILE', 'missing.zip');
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(404);
    });

    it('returns 404 when DOWNLOADS_DIR is not configured', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        vi.stubEnv('DOWNLOADS_DIR', undefined);
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(404);
    });

    it('returns 404 when GAME_CLIENT_FILE is not configured', async () => {
        db.getLoginFromToken.mockResolvedValue('user1');
        vi.stubEnv('GAME_CLIENT_FILE', undefined);
        const cookies = { get: () => EXAMPLE_SESSION_TOKEN };

        const response = await api.GET({ cookies });

        expect(response.status).toBe(404);
    });
});
