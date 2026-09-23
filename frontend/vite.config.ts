import { fileURLToPath } from "node:url";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));

const config = defineConfig(({ mode }) => {
	const env = loadEnv(mode, repositoryRoot, "");
	const apiTarget =
		process.env.VITE_API_PROXY_TARGET ||
		env.VITE_API_PROXY_TARGET ||
		"http://127.0.0.1:8080";

	return {
		optimizeDeps: {
			include: ["@mantine/form"],
		},
		resolve: { tsconfigPaths: true },
		plugins: [
			devtools(),
			tanstackRouter({
				target: "react",
				autoCodeSplitting: true,
				codeSplittingOptions: {
					splitBehavior: ({ routeId }) =>
						routeId === "/instruments/$symbol" ? [] : undefined,
				},
			}),
			viteReact(),
			mode === "development" && {
				name: "telegram-development-init-data",
				transformIndexHtml() {
					return [
						{
							children: `window.location.hash = ${JSON.stringify(`tgWebAppData=${encodeURIComponent(env.TELEGRAM_DEV_INIT_DATA.trim())}`)};`,
							injectTo: "head-prepend",
							tag: "script",
						},
					];
				},
			},
		],
		server: {
			proxy: {
				"/api": {
					target: apiTarget,
					ws: true,
				},
				"/health": {
					target: apiTarget,
				},
			},
		},
	};
});

export default config;
