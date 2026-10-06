import { Button, Paper, SimpleGrid, Stack, useMatches } from "@mantine/core";
import { useControlSize } from "@/app/use-control-size";
import { NumberInputFieldset } from "@/components/number-input-fieldset";
import type { SettingsFormProps } from "@/components/settings-form/types";

export function SettingsForm({
	columns = 1,
	groups,
	disabled,
	loading,
	onSubmit,
	submitLabel,
}: SettingsFormProps) {
	const controlSize = useControlSize();
	const spacing = useMatches({ base: "xs", sm: "sm" });
	const padding = useMatches({ base: "xs", sm: "md" });

	return (
		<Paper component="form" onSubmit={onSubmit} p={padding}>
			<Stack gap={spacing}>
				<SimpleGrid cols={columns} spacing={spacing}>
					{groups.map((group) => (
						<NumberInputFieldset
							key={group.id}
							title={group.title}
							inputs={group.inputs}
						/>
					))}
				</SimpleGrid>
				<Button
					disabled={disabled}
					loading={loading}
					size={controlSize}
					type="submit"
				>
					{submitLabel}
				</Button>
			</Stack>
		</Paper>
	);
}
