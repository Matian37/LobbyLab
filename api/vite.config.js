import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import path from 'node:path';
import { websocketLogger } from './src/lib/logger.js';

// Track servers we've already attached to, so restarts don't double-bind
const attachedServers = new WeakSet();

async function attachWebSocketServer(server, loadCreateWssFunc) {
    const httpServer = server.httpServer;

    if (!httpServer) return;
    if (attachedServers.has(httpServer)) return;
    attachedServers.add(httpServer);

    try {
        let createWebSocketServer = await loadCreateWssFunc();

        const wss = await createWebSocketServer(httpServer);
        websocketLogger.info('websocket server attached');

        httpServer.once('close', () => {
            websocketLogger.info('websocket server closing');
            wss.close();
            attachedServers.delete(httpServer);
        });
    } catch (err) {
        attachedServers.delete(httpServer);
        websocketLogger.error({ err }, 'websocket server failed to attach');
    }
}

function websocketServer() {
    return {
        name: 'websocket-server',
        configureServer(server) {
            return async () => {
                await attachWebSocketServer(server, async () => {
                    const module = await server.ssrLoadModule(
                        '$lib/server/server.js'
                    );
                    return module.createWebSocketServer;
                });
            };
        },
        configurePreviewServer(server) {
            return async () => {
                await attachWebSocketServer(server, async () => {
                    const modulePath = path.resolve(
                        import.meta.dirname,
                        './src/lib/server/server.js'
                    );
                    const module = await import(modulePath);
                    return module.createWebSocketServer;
                });
            };
        },
    };
}

export default defineConfig({
    plugins: [sveltekit(), websocketServer()],
    test: {
        environment: 'jsdom',
        env: {
            LOG_LEVEL: 'silent',
        },
        chaiConfig: { truncateThreshold: 0 },
    },
    resolve: {
        conditions: process.env.VITEST ? ['browser'] : undefined,
        alias: {
            $lib: path.resolve(import.meta.dirname, './src/lib'),
            $routes: path.resolve(import.meta.dirname, './src/routes'),
            ...(process.env.VITEST && { ws: import.meta.resolve('ws') }),
        },
    },
});
