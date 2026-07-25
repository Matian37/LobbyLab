import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	test: {
		environment: 'jsdom',
		alias: {
			$lib : new URL('./src/lib', import.meta.url).pathname,
			$routes: new URL('./src/routes', import.meta.url).pathname,
		},
		globalSetup: './tests/global-setup.js'
	},
	resolve: process.env.VITEST ? { conditions: ['browser'] } : undefined
});
