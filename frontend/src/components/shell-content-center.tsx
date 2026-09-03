import { Center } from "@mantine/core";
import type { PropsWithChildren } from "react";

export function ShellContentCenter({ children }: PropsWithChildren) {
	return <Center mih="calc(100dvh - 5rem)">{children}</Center>;
}
