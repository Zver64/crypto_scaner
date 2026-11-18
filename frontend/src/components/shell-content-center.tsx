import { Center } from "@mantine/core";
import type { PropsWithChildren } from "react";

import { usePageViewport } from "@/components/page-viewport";

export function ShellContentCenter({ children }: PropsWithChildren) {
	return <Center mih={usePageViewport().height}>{children}</Center>;
}
