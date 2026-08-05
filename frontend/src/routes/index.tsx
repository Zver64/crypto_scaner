import { Progress } from "@mantine/core";
import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
	return (
		<div className="p-8">
			<h1 className="text-4xl font-bold">Frontend template</h1>
      <div>here is the mantine progress bar test</div>
      <Progress value={10}></Progress>
		</div>
	);
}
