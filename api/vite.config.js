import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';

// Track servers we've already attached to, so restarts don't double-bind
const attachedServers = new WeakSet();

function attachWebSocketServer(server) {
    const httpServer = server.httpServer;

    if (!httpServer) return;
    if (attachedServers.has(httpServer)) return;
    attachedServers.add(httpServer);

    return server
        .ssrLoadModule('$lib/connection.js')
        .then(async ({ createWebSocketServer }) => {
            const wss = await createWebSocketServer(httpServer);
            httpServer.once('close', () => {
                console.log('[websocket-server] closing');
                wss.close();
                attachedServers.delete(httpServer);
            });
        })
        .catch((err) => {
            attachedServers.delete(httpServer);
            console.error('[websocket-server] failed to attach:', err);
        });
}

function websocketServer() {
    return {
        name: 'websocket-server',
        configureServer(server) {
            return async () => {
                await attachWebSocketServer(server);
            };
        },
        configurePreviewServer(server) {
            return async () => {
                await attachWebSocketServer(server);
            };
        },
    };
}

const alias = {
    $lib: fileURLToPath(new URL('./src/lib', import.meta.url)),
    $routes: fileURLToPath(new URL('./src/routes', import.meta.url)),
};

export default defineConfig({
    plugins: [sveltekit(), websocketServer()],
    test: {
        environment: 'jsdom',
        alias,
        chaiConfig: { truncateThreshold: 0 },
    },
    resolve: {
        alias,
        conditions: process.env.VITEST ? ['browser'] : undefined,
    },
});
