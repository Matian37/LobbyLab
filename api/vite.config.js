import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';
import { websocketLogger } from './src/lib/logger.js';

// Track servers we've already attached to, so restarts don't double-bind
const attachedServers = new WeakSet();

function attachWebSocketServer(server) {
    const httpServer = server.httpServer;

    if (!httpServer) return;
    if (attachedServers.has(httpServer)) return;
    attachedServers.add(httpServer);

    return server
        .ssrLoadModule('$lib/server/server.js')
        .then(async ({ createWebSocketServer }) => {
            const wss = await createWebSocketServer(httpServer);
            websocketLogger.info('websocket server attached');
            httpServer.once('close', () => {
                websocketLogger.info('websocket server closing');
                wss.close();
                attachedServers.delete(httpServer);
            });
        })
        .catch((err) => {
            attachedServers.delete(httpServer);
            websocketLogger.error({ err }, 'websocket server failed to attach');
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
        env: {
            LOG_LEVEL: 'silent',
        },
        chaiConfig: { truncateThreshold: 0 },
    },
    resolve: {
        alias,
    },
});
