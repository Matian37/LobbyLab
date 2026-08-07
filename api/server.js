import http from 'node:http';
import { handler } from './build/handler.js';
import { createWebSocketServer } from './src/lib/server/server.js';
import { httpLogger } from './src/lib/logger.js';

const server = http.createServer(handler);
const wss = await createWebSocketServer(server);

const port = process.env.PORT;
if (!port) {
    httpLogger.fatal('PORT environment variable is required');
    process.exit(1);
}
server.listen(port);

server.on('listening', () => {
    httpLogger.info({ port }, 'api server listening');
});

server.on('error', (err) => {
    httpLogger.error({ err }, 'api server failed');
});

for (const sig of ['SIGTERM', 'SIGINT']) {
    process.once(sig, async () => {
        httpLogger.info({ sig }, 'shutdown signal received');
        await wss.close();
        server.close(() => {
            httpLogger.info('api server closed gracefully');
            process.exit(0);
        });
    });
}
