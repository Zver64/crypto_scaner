import {
	Button,
	Fieldset,
	Flex,
	Group,
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
	title: string;
}

export function NumberInputFieldset({
	inputs,
	title,
}: NumberInputFieldsetProps) {
	return (
		<Fieldset legend={title}>
			<Stack gap="xs">
				{inputs.map(({ presets, ...inputProps }) => {
					const numberInput = (
						<NumberInput key={inputProps.id} {...inputProps} />
					);

					if (!presets?.length) return numberInput;

					return (
						<Flex
							align={{ base: "stretch", sm: "end" }}
							direction={{ base: "column", sm: "row" }}
							gap="xs"
							key={inputProps.id}
						>
							<div style={{ flex: 1 }}>{numberInput}</div>
							<Group gap="xs" justify="center">
								{presets.map((preset) => (
									<Button
										key={String(preset.value)}
										onClick={(event) => {
											inputProps.onChange?.(preset.value);
											event.currentTarget.blur();
										}}
										size="sm"
										type="button"
										variant="default"
									>
										{preset.label}
									</Button>
								))}
							</Group>
						</Flex>
					);
				})}
			</Stack>
		</Fieldset>
	);
}
