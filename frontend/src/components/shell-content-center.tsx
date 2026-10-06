import { Center } from "@mantine/core";
import type { PropsWithChildren } from "react";
import { usePageViewport } from "@/app/page-height";

export function ShellContentCenter({ children }: PropsWithChildren) {
	return <Center mih={usePageViewport().height}>{children}</Center>;
}
