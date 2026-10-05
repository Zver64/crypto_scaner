import {
	Box,
	Group,
	Paper,
	Stack,
	Text,
	Title,
	useMantineTheme,
} from "@mantine/core";
import { themeToVars } from "@mantine/vanilla-extract";
import type Decimal from "decimal.js";
import { LegendItem } from "@/features/instrument-analysis/grid-estimator/legend-item";
import {
	formatAmount,
	liquidationRangeBar,
} from "@/features/instrument-analysis/grid-estimator/utils";
import type { FuturesGridEstimate } from "@/utils/calculator/futures-grid";

interface LiquidationRangeBarProps {
	currentPrice: number | null;
	estimate: FuturesGridEstimate;
	priceUnit: string;
}

const MARKER_WIDTH = 3;

// Shows the grid range, the current price, and the liquidation price on one
// scale, so it is clear how close the liquidation sits to the grid.
export function LiquidationRangeBar({
	currentPrice,
	estimate,
	priceUnit,
}: LiquidationRangeBarProps) {
	const formatPrice = (price: Decimal | number) =>
		formatAmount(price, priceUnit);
	const { colors } = themeToVars(useMantineTheme());
	const { liquidationPrice, lowerPrice, upperPrice } = estimate;
	const bar = liquidationRangeBar(
		lowerPrice,
		upperPrice,
		currentPrice,
		liquidationPrice,
	);
	const grid = `${formatPrice(lowerPrice)} – ${formatPrice(upperPrice)}`;
	const liquidation = liquidationPrice ? formatPrice(liquidationPrice) : null;
	const current = currentPrice === null ? null : formatPrice(currentPrice);
	const markers = [
		{ color: colors.text, key: "current", position: bar.currentPosition },
		{
			color: colors.red[6],
			key: "liquidation",
			position: bar.liquidationPosition,
		},
	];

	return (
		<Paper withBorder radius="md" p="md" role="group" aria-label="Liquidation">
			<Stack gap="sm">
				<Group justify="space-between" wrap="wrap">
					<Title order={3} size="md">
						Liquidation
					</Title>
					<Text c={bar.isInsideGrid ? "red" : "dimmed"} size="sm">
						{bar.summary}
					</Text>
				</Group>
				<Box
					aria-label={`Grid ${grid}${current ? `, current price ${current}` : ""}${
						liquidation ? `, liquidation price ${liquidation}` : ""
					}. ${bar.summary}`}
					bg={colors.defaultHover}
					h={14}
					pos="relative"
					role="img"
					w="100%"
				>
					<Box
						bg={colors.teal[6]}
						h="100%"
						left={`${bar.gridStartPosition}%`}
						pos="absolute"
						w={`${bar.gridEndPosition - bar.gridStartPosition}%`}
					/>
					{markers.map(({ color, key, position }) =>
						position === null ? null : (
							<Box
								bg={color}
								h="100%"
								key={key}
								left={`calc(${position}% - ${MARKER_WIDTH / 2}px)`}
								pos="absolute"
								w={MARKER_WIDTH}
							/>
						),
					)}
				</Box>
				<Group gap="md" justify="space-between" wrap="wrap">
					<LegendItem color={colors.teal[6]} label="Grid" value={grid} />
					<LegendItem
						color={colors.red[6]}
						label="Liquidation"
						value={liquidation ?? "None"}
					/>
				</Group>
			</Stack>
		</Paper>
	);
}
