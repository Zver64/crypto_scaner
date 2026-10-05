import {
	createSeriesMarkers,
	type ISeriesMarkersPluginApi,
	type SeriesMarker,
	type Time,
} from "lightweight-charts";
import { useLayoutEffect, useRef } from "react";
import { useSeriesLifecycle } from "@/components/lightweight-chart/context";

interface SeriesMarkersProps {
	markers: readonly SeriesMarker<Time>[];
}

export function SeriesMarkers({ markers }: SeriesMarkersProps) {
	const parent = useSeriesLifecycle();
	const markersRef = useRef(markers);
	const pluginRef = useRef<ISeriesMarkersPluginApi<Time> | null>(null);
	useLayoutEffect(() => {
		markersRef.current = markers;
	}, [markers]);

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
		pluginRef.current?.setMarkers(Array.from(markers));
	}, [markers]);

	return null;
}
