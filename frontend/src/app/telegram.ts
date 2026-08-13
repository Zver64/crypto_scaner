import { useEffect, useRef, useState } from "react";

interface TelegramBackButton {
	hide(): void;
	offClick(listener: () => void): void;
	onClick(listener: () => void): void;
	show(): void;
}

interface TelegramSafeAreaInsets {
	bottom: number;
	left: number;
	right: number;
	top: number;
}

interface TelegramWebApp {
	BackButton?: TelegramBackButton;
	contentSafeAreaInset?: TelegramSafeAreaInsets;
	expand(): void;
	initData: string;
	isVersionAtLeast?(version: string): boolean;
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

export function useTelegramBackButton(onBack: () => void) {
	const webApp = window.Telegram?.WebApp;
	const backButton =
		webApp?.BackButton && (webApp.isVersionAtLeast?.("6.1") ?? true)
			? webApp.BackButton
			: undefined;
	const onBackRef = useRef(onBack);
	onBackRef.current = onBack;

	useEffect(() => {
		if (!backButton) {
			return;
		}

		const handleClick = () => onBackRef.current();
		backButton.onClick(handleClick);
		backButton.show();

		return () => {
			backButton.offClick(handleClick);
			backButton.hide();
		};
	}, [backButton]);

	return backButton !== undefined;
}
