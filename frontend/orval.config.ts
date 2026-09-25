import { defineConfig } from "orval";

export default defineConfig({
	cryptoScanner: {
		input: "../backend/internal/httpapi/openapi/openapi.yaml",
		output: {
			client: "react-query",
			mode: "single",
			target: "./src/api/generated/api.ts",
			schemas: "./src/api/generated/models",
			// Remove models of schemas that no longer exist in the contract.
			clean: true,
			urlEncodeParameters: true,
			override: {
				fetch: {
					// Reject non-2xx responses before TanStack Query can cache them as successful data.
					forceSuccessResponse: true,
				},
				query: {
					shouldExportKeys: true,
				},
				operations: {
					analyzeInstrument: {
						query: {
							useMutation: false,
							useQuery: true,
						},
					},
					analyzeMarket: {
						query: {
							useMutation: false,
							useQuery: true,
						},
					},
					analyzeFavorites: {
						query: {
							useMutation: false,
							useQuery: true,
						},
					},
					listInstrumentCandles: {
						query: {
							useInfinite: true,
							useInfiniteQueryParam: "before",
							useQuery: false,
						},
					},
				},
			},
		},
	},
});
