import { server } from './build/index.js';
import { createWebSocketServer } from './src/lib/connection.js';

createWebSocketServer(server.server);
