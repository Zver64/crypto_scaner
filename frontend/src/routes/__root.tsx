import { TanStackDevtools } from "@tanstack/react-devtools";
import { createRootRouteWithContext } from "@tanstack/react-router";
import { TanStackRouterDevtoolsPanel } from "@tanstack/react-router-devtools";
import { MiniAppShell } from "@/app/app-shell";

// Routes may set pageTitle in beforeLoad; the app header shows the deepest one.
export interface RouterContext {
	pageTitle?: string;
}

export const Route = createRootRouteWithContext<RouterContext>()({
	component: RootComponent,
});

function RootComponent() {
	return (
		<>
			<MiniAppShell />
			<TanStackDevtools
				config={{
					position: "bottom-right",
				}}
				plugins={[
					{
						name: "TanStack Router",
						render: <TanStackRouterDevtoolsPanel />,
					},
				]}
			/>
		</>
	);
}
