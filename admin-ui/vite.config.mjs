import {defineConfig} from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  base: "/admin/",
  plugins: [react()],
  // Candidate only: promote to embedded dist after production workflow gates.
  build: {
    outDir: "build-candidate", emptyOutDir: true, sourcemap: false,
    rollupOptions: {output: {manualChunks(id) {
      // Stable framework bytes can be cached across application-only releases.
      // This is cache separation, not a claim of reduced first-load bytes.
      if (/\/node_modules\/(?:react|react-dom|scheduler)\//.test(id.replaceAll("\\", "/"))) return "react-vendor";
    }}},
  },
  server: {strictPort: true, port: 18225},
  preview: {strictPort: true, port: 18226},
});
