import { fileURLToPath } from "node:url";
import { devtools } from "@tanstack/devtools-vite";
import { tanstackRouter } from "@tanstack/router-plugin/vite";
import viteReact from "@vitejs/plugin-react";
import { defineConfig, loadEnv } from "vite";

const repositoryRoot = fileURLToPath(new URL("..", import.meta.url));

export function developmentAuthorizationHeader(initData: string | undefined) {
	const value = initData?.trim();
	return value ? `tma ${value}` : undefined;
}

const config = defineConfig(({ mode }) => {
	const env = loadEnv(mode, repositoryRoot, "");
	const apiTarget =
		process.env.VITE_API_PROXY_TARGET ||
		env.VITE_API_PROXY_TARGET ||
		"http://127.0.0.1:8080";
	const developmentAuthorization = developmentAuthorizationHeader(
		env.TELEGRAM_DEV_INIT_DATA,
	);

	return {
		optimizeDeps: {
			include: ["@mantine/form"],
		},
		resolve: { tsconfigPaths: true },
		plugins: [
			devtools(),
			tanstackRouter({ target: "react", autoCodeSplitting: true }),
			viteReact(),
		],
		server: {
			proxy: {
				"/api": {
					target: apiTarget,
					configure(proxy) {
						proxy.on("proxyReq", (proxyRequest) => {
							if (
								developmentAuthorization &&
								!proxyRequest.hasHeader("authorization")
							) {
								proxyRequest.setHeader(
									"authorization",
									developmentAuthorization,
								);
							}
						});
					},
				},
				"/health": {
					target: apiTarget,
				},
			},
		},
	};
});

export default config;
