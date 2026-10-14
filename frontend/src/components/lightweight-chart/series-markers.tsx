import {
	createSeriesMarkers,
	type ISeriesMarkersPluginApi,
	type SeriesMarker,
	type Time,
} from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";

import { useSeriesLifecycle } from "@/components/lightweight-chart/context";

interface SeriesMarkersProps {
	// Applied when the reference changes: the data boundary creates a new array
	// only when the markers change.
	markers: readonly SeriesMarker<Time>[];
}

export function SeriesMarkers({ markers }: SeriesMarkersProps) {
	const parent = useSeriesLifecycle();
	// The markers the plugin has: it is created with them.
	const markersRef = useRef(markers);
	const pluginRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);

	useLayoutEffect(() => {
		pluginRef.current = createSeriesMarkers(
			parent.api(),
			Array.from(markersRef.current),
		);
		return () => {
			const plugin = pluginRef.current;
			pluginRef.current = null;
			if (plugin !== null && !parent.isRemoved) plugin.detach();
		};
	}, [parent]);

	useLayoutEffect(() => {
		if (markersRef.current === markers) return;
		markersRef.current = markers;
		pluginRef.current?.setMarkers(Array.from(markers));
	});

	return null;
}
