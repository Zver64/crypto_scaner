import type { DefaultMantineColor, MantineTheme } from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";

// The named Mantine palettes. DefaultMantineColor also admits any string,
// which this leaves out, so a misspelled name fails typecheck.
type PaletteName = DefaultMantineColor extends infer Name
	? Name extends string
		? string extends Name
			? never
			: Name
		: never
	: never;

// One sequence for groups and functions alike. Each palette is roughly
// 137° around the color wheel from the previous one, so every level differs
// as much from its parent as any other. Red is left out, since it marks
// removal and errors.
const palettes = [
	"blue",
	"pink",
	"green",
	"violet",
	"orange",
	"teal",
	"grape",
	"lime",
	"cyan",
	"yellow",
] as const satisfies readonly PaletteName[];

// The border color of a group or function nested depth levels deep, counting
// groups and functions together; deeper levels repeat the sequence.
export function nestingBorder(theme: MantineTheme, depth: number) {
	return themeToVars(theme).colors[palettes[depth % palettes.length]].outline;
}
