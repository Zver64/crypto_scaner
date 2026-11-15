import { useLayoutEffect, useRef } from "react";

import { useSeriesLifecycle } from "@/components/lightweight-chart/context";
import {
	createFillPrimitive,
	type FillPrimitive,
} from "@/components/lightweight-chart/fill-primitive";
import type { SeriesFillPoint } from "@/components/lightweight-chart/types";

interface SeriesFillProps {
	// Applied when the reference changes.
	points: readonly SeriesFillPoint[];
}

// A translucent area on the price scale of the series, behind it.
export function SeriesFill({ points }: SeriesFillProps) {
	const parent = useSeriesLifecycle();
	// The points the primitive has: it is created with them.
	const pointsRef = useRef(points);
	const primitiveRef = useRef<FillPrimitive | null>(null);

	useLayoutEffect(() => {
		const series = parent.api();
		const primitive = createFillPrimitive(pointsRef.current);
		series.attachPrimitive(primitive);
		primitiveRef.current = primitive;
		return () => {
			primitiveRef.current = null;
			if (!parent.isRemoved) series.detachPrimitive(primitive);
		};
	}, [parent]);

	useLayoutEffect(() => {
		if (pointsRef.current === points) return;
		pointsRef.current = points;
		primitiveRef.current?.setPoints(points);
	});

	return null;
}
