import { Button, Progress } from "@mantine/core";
import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";

export const Route = createFileRoute("/")({ component: Home });

function Home() {
	const [status, setStatus] = useState<string | null>(null);
	const handleCheck = async () => {
		const data = await fetch("/health/live").then((res) => res.json());
		console.log("data: ", data);
		setStatus(data.status);
		const ready = await fetch("/health/ready").then((res) => res.json());
		console.log("ready: ", ready);
	};
	return (
		<div className="p-8">
			<h1 className="text-4xl font-bold">Frontend template</h1>
			<div>here is the mantine progress bar test</div>
			<div>status is: {status}</div>
			<Button onClick={handleCheck}>chech health</Button>
			<Progress value={10}></Progress>
		</div>
	);
}
