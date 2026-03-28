import { defineConfig } from 'vite';

export default defineConfig({
    resolve: {
        alias: {
            '@wailsio/runtime': '/wails/runtime.js'
        }
    },
    build: {
        rollupOptions: {
            external: (id) => id.startsWith('/wails/')
        }
    }
});
