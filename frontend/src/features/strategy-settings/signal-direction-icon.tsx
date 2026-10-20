import { useMantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import {
	IconArrowsVertical,
	IconCircleFilled,
	IconSpeakerphone,
} from "@tabler/icons-react";

import type { Direction } from "@/api/generated/models";

// The marks of the Telegram signal alerts: 📣, 🟢, 🔴, and ↕️.
const directionIcons = {
	simple: IconSpeakerphone,
	long: IconCircleFilled,
	short: IconCircleFilled,
	sideways: IconArrowsVertical,
} as const satisfies Record<Direction, typeof IconCircleFilled>;

interface SignalDirectionIconProps {
	direction: Direction;
}

// The move a signal expects.
export function SignalDirectionIcon({ direction }: SignalDirectionIconProps) {
	const { colors } = themeToVars(useMantineTheme());
	const Icon = directionIcons[direction];
	const color = {
		simple: colors.orange[6],
		long: colors.green[6],
		short: colors.red[6],
		sideways: colors.blue[6],
	}[direction];
	return <Icon color={color} size={16} style={{ flexShrink: 0 }} />;
}
