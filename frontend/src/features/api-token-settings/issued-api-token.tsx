import { Alert, Button, Code, CopyButton, Group, Modal } from "@mantine/core";

interface IssuedApiTokenProps {
	onDismiss(): void;
	// The plaintext token; the dialog is open while it is set.
	token: string | undefined;
}

// Shows a new token once; it is not kept anywhere after dismissing.
export function IssuedApiToken({ onDismiss, token }: IssuedApiTokenProps) {
	return (
		<Modal
			centered
			closeOnClickOutside={false}
			closeOnEscape={false}
			onClose={onDismiss}
			opened={token !== undefined}
			title="API token created"
		>
			<Alert color="yellow" variant="light">
				Copy the token now. It is shown only once and cannot be seen again.
			</Alert>
			<Code
				block
				mt="md"
				style={{ wordBreak: "break-all", whiteSpace: "pre-wrap" }}
			>
				{token}
			</Code>
			<Group justify="flex-end" mt="md">
				<CopyButton timeout={1500} value={token ?? ""}>
					{({ copied, copy }) => (
						<Button
							color={copied ? "teal" : undefined}
							onClick={copy}
							variant="light"
						>
							{copied ? "Copied" : "Copy"}
						</Button>
					)}
				</CopyButton>
				<Button onClick={onDismiss}>Done</Button>
			</Group>
		</Modal>
	);
}
