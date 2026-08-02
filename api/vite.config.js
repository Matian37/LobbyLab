import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import { fileURLToPath } from 'node:url';

// Track servers we've already attached to, so restarts don't double-bind
const attachedServers = new WeakSet();

function attachWebSocketServer(httpServer) {
    if (!httpServer) return;
    if (attachedServers.has(httpServer)) return;
    attachedServers.add(httpServer);

    return import('./src/lib/connection.js')
        .then(async ({ createWebSocketServer }) => {
            await createWebSocketServer(httpServer);
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
                await attachWebSocketServer(server.httpServer);
            };
        },
        configurePreviewServer(server) {
            return async () => {
                await attachWebSocketServer(server.httpServer);
            };
        },
    };
}

export default defineConfig({
    plugins: [sveltekit(), websocketServer()],
    test: {
        environment: 'jsdom',
        alias: {
            $lib: fileURLToPath(new URL('./src/lib', import.meta.url)),
            $routes: fileURLToPath(new URL('./src/routes', import.meta.url)),
        },
        chaiConfig: {
            truncateThreshold: 0,
        },
    },
    resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined,
});
