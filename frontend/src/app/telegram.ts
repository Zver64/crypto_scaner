import { useEffect, useState } from "react";

interface TelegramSafeAreaInsets {
	bottom: number;
	left: number;
	right: number;
	top: number;
}

interface TelegramWebApp {
	contentSafeAreaInset?: TelegramSafeAreaInsets;
	expand(): void;
	initData: string;
	offEvent?(
		event: "contentSafeAreaChanged" | "safeAreaChanged",
		listener: () => void,
	): void;
	onEvent?(
		event: "contentSafeAreaChanged" | "safeAreaChanged",
		listener: () => void,
	): void;
	ready(): void;
	safeAreaInset?: TelegramSafeAreaInsets;
}

declare global {
	interface Window {
		Telegram?: { WebApp?: TelegramWebApp };
	}
}

const emptyInsets: TelegramSafeAreaInsets = {
	bottom: 0,
	left: 0,
	right: 0,
	top: 0,
};

const initializedWebApps = new WeakSet<TelegramWebApp>();

function getSafeAreaInsets(webApp: TelegramWebApp | undefined) {
	const safeArea = webApp?.safeAreaInset ?? emptyInsets;
	const contentSafeArea = webApp?.contentSafeAreaInset ?? emptyInsets;

	return {
		bottom: Math.max(safeArea.bottom, contentSafeArea.bottom),
		left: Math.max(safeArea.left, contentSafeArea.left),
		right: Math.max(safeArea.right, contentSafeArea.right),
		top: Math.max(safeArea.top, contentSafeArea.top),
	};
}

export function useTelegramMiniApp() {
	const webApp = window.Telegram?.WebApp;
	const [safeAreaInsets, setSafeAreaInsets] = useState(() =>
		getSafeAreaInsets(webApp),
	);

	useEffect(() => {
		if (!webApp) {
			return;
		}

		if (!initializedWebApps.has(webApp)) {
			webApp.ready();
			webApp.expand();
			initializedWebApps.add(webApp);
		}

		const updateSafeArea = () => setSafeAreaInsets(getSafeAreaInsets(webApp));
		updateSafeArea();
		webApp.onEvent?.("safeAreaChanged", updateSafeArea);
		webApp.onEvent?.("contentSafeAreaChanged", updateSafeArea);

		return () => {
			webApp.offEvent?.("safeAreaChanged", updateSafeArea);
			webApp.offEvent?.("contentSafeAreaChanged", updateSafeArea);
		};
	}, [webApp]);

	return { safeAreaInsets, webApp };
}

export function getTelegramInitData() {
	return window.Telegram?.WebApp?.initData;
}
