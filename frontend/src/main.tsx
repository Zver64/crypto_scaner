import { createTheme, MantineProvider } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider } from "@tanstack/react-router";
import ReactDOM from "react-dom/client";
import { getRouter } from "@/router";

import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";

const theme = createTheme({
	defaultRadius: "md",
	fontFamily:
		"Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, sans-serif",
	primaryColor: "teal",
});

const queryClient = new QueryClient();

const router = getRouter();

const rootElement = document.getElementById("app")!;

if (!rootElement.innerHTML) {
	const root = ReactDOM.createRoot(rootElement);
	root.render(
		<QueryClientProvider client={queryClient}>
			<MantineProvider
				defaultColorScheme="dark"
				forceColorScheme="dark"
				theme={theme}
			>
				<Notifications position="top-center" />
				<RouterProvider router={router} />
			</MantineProvider>
		</QueryClientProvider>,
	);
}
