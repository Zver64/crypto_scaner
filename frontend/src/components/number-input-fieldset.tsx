import {
	Button,
	Fieldset,
	Flex,
	Group,
	Input,
	NumberInput,
	type NumberInputProps,
	Stack,
} from "@mantine/core";

type NumberInputPreset = Required<Pick<NumberInputProps, "value">> & {
	label: string;
};

export type NumberInputField = NumberInputProps & {
	id: string;
	presets?: readonly NumberInputPreset[];
};

interface NumberInputFieldsetProps {
	inputs: readonly NumberInputField[];
	presetsPosition?: "below" | "right";
	title: string;
}

export function NumberInputFieldset({
	inputs,
	presetsPosition = "right",
	title,
}: NumberInputFieldsetProps) {
	return (
		<Fieldset legend={title}>
			<Stack gap="xs">
				{inputs.map(({ presets, ...inputProps }) => {
					if (!presets?.length) {
						return <NumberInput key={inputProps.id} {...inputProps} />;
					}

					const { label, ...numberInputProps } = inputProps;

					return (
						<Stack gap={2} key={inputProps.id}>
							{label && (
								<Input.Label htmlFor={inputProps.id}>{label}</Input.Label>
							)}
							<Flex
								direction={presetsPosition === "below" ? "column" : "row"}
								gap="xs"
							>
								<div style={{ flex: 1 }}>
									<NumberInput {...numberInputProps} />
								</div>
								<Group gap="xs" justify="center">
									{presets.map((preset) => (
										<Button
											key={String(preset.value)}
											onClick={(event) => {
												inputProps.onChange?.(preset.value);
												event.currentTarget.blur();
											}}
											size="xs"
											type="button"
											variant="default"
										>
											{preset.label}
										</Button>
									))}
								</Group>
							</Flex>
						</Stack>
					);
				})}
			</Stack>
		</Fieldset>
	);
}
