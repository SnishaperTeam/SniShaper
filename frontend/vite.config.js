import { defineConfig } from 'vite';

export default defineConfig({
    build: {
        chunkSizeWarningLimit: 600,
        rollupOptions: {
            external: (id) => id.startsWith('/wails/'),
            onwarn(warning, warn) {
                if (warning.code === 'MODULE_LEVEL_DIRECTIVE' && warning.message.includes("'use client'")) {
                    return;
                }
                warn(warning);
            }
        }
    }
});
