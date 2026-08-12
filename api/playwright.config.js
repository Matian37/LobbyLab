import { defineConfig } from '@playwright/test';
import {
    BASE_URL,
    DATABASE_URL,
    DOWNLOADS_DIR,
    CLIENT_FILE,
} from './e2e/helpers.js';

export default defineConfig({
    testDir: 'e2e',
    testMatch: '**/*.e2e.js',
    timeout: 60_000,
    workers: 1,
    fullyParallel: false,
    reporter: [['list']],
    webServer: {
        command: 'npm run preview',
        url: BASE_URL,
        reuseExistingServer: !process.env.CI,
        timeout: 90_000,
        gracefulShutdown: {
            signal: 'SIGTERM',
            timeout: 15_000,
        },
        env: {
            DATABASE_URL,
            DOWNLOADS_DIR,
            GAME_CLIENT_FILE: CLIENT_FILE,
            ORIGIN: BASE_URL,
            PUBLIC_GAME_LAUNCH_URL: 'http://game/{host}/{port}/{token}',
            LOG_LEVEL: 'silent',
        },
    },
    use: {
        headless: true,
    },
});
