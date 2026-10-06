import { MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import ReactDOM from "react-dom/client";
import { theme } from "@/app/theme";
import { getRouter } from "@/router";

import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import "@microcharts/react/styles.css";
import "@/styles.css";

const queryClient = new QueryClient();

const router = getRouter();

const rootElement = document.getElementById("app");
if (!rootElement) {
	throw new Error("The #app root element is missing from index.html.");
}

if (!rootElement.innerHTML) {
	const root = ReactDOM.createRoot(rootElement);
	root.render(
		<QueryClientProvider client={queryClient}>
			<MantineProvider defaultColorScheme="dark" theme={theme}>
				<Notifications position="top-center" />
				<RouterProvider router={router} />
			</MantineProvider>
		</QueryClientProvider>,
	);
}
