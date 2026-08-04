import http from 'node:http';
import { handler } from './build/handler.js';
import { createWebSocketServer } from './src/lib/server.js';

const server = http.createServer(handler);
const wss = await createWebSocketServer(server);

const port = process.env.PORT;
if (!port) {
    throw new Error('PORT environment variable is required');
}
server.listen(port);

for (const sig of ['SIGTERM', 'SIGINT']) {
    process.once(sig, async () => {
        await wss.close();
        server.close(() => process.exit(0));
    });
}
