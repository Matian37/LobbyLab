import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import { Readable } from 'node:stream';
import path from 'node:path';
import { getLoginFromToken } from '$lib/db.js';
import { ERRORS } from '$lib/errors.js';
import { validateSession } from '$lib/validate.js';

export async function GET({ cookies }) {
    const result = validateSession(cookies);
    if (result.error !== undefined) return result.error;

    const login = await getLoginFromToken(result.data.token);
    if (login === null) return ERRORS.invalidSessionToken();

    const dir = process.env.DOWNLOADS_DIR;
    if (dir === undefined) {
        return ERRORS.notFound();
    }

    const fileName = process.env.GAME_CLIENT_FILE;
    if (fileName === undefined) {
        return ERRORS.notFound();
    }

    const filePath = path.join(dir, fileName);

    try {
        const fileStat = await stat(filePath);
        if (!fileStat.isFile()) {
            return ERRORS.notFound();
        }

        const stream = Readable.toWeb(createReadStream(filePath));
        return new Response(stream, {
            headers: {
                'Content-Type': 'application/octet-stream',
                'Content-Length': String(fileStat.size),
                'Content-Disposition': `attachment; filename="${fileName}"`,
            },
        });
    } catch (error) {
        if (error.code === 'ENOENT') {
            return ERRORS.notFound();
        }
        throw error;
    }
}
