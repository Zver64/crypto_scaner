import { useEffect, useRef } from "react";

interface TelegramBackButton {
	hide(): void;
	offClick(listener: () => void): void;
	onClick(listener: () => void): void;
	show(): void;
}

interface TelegramWebApp {
	BackButton?: TelegramBackButton;
	disableVerticalSwipes?(): void;
	expand(): void;
	initData: string;
	isVersionAtLeast?(version: string): boolean;
	openLink?(url: string): void;
	ready(): void;
}

declare global {
	interface Window {
		Telegram?: { WebApp?: TelegramWebApp };
	}
}

const initializedWebApps = new WeakSet<TelegramWebApp>();

export function initializeTelegramMiniApp(webApp: TelegramWebApp) {
	if (initializedWebApps.has(webApp)) {
		return;
	}

	webApp.ready();
	webApp.expand();
	if (webApp.isVersionAtLeast?.("7.7")) {
		webApp.disableVerticalSwipes?.();
	}
	initializedWebApps.add(webApp);
}

export function useTelegramMiniApp() {
	const webApp = window.Telegram?.WebApp;

	useEffect(() => {
		if (webApp) {
			initializeTelegramMiniApp(webApp);
		}
	}, [webApp]);

	return { webApp };
}

export function getTelegramInitData() {
	if (typeof window === "undefined") return undefined;
	return window.Telegram?.WebApp?.initData;
}

/** Telegram user ID that init data claims; the backend verifies it. */
export function telegramUserID(
	initData = getTelegramInitData(),
): number | undefined {
	if (!initData) return undefined;
	try {
		const rawUser = new URLSearchParams(initData).get("user");
		if (!rawUser) return undefined;
		const user = JSON.parse(rawUser) as { id?: unknown };
		return typeof user.id === "number" && Number.isSafeInteger(user.id)
			? user.id
			: undefined;
	} catch {
		return undefined;
	}
}

export function openTelegramExternalLink(url: string): boolean {
	if (typeof window === "undefined") {
		return false;
	}

	const openLink = window.Telegram?.WebApp?.openLink;
	if (!openLink) {
		return false;
	}

	openLink(url);
	return true;
}

function telegramBackButton() {
	const webApp = window.Telegram?.WebApp;
	return webApp?.BackButton && (webApp.isVersionAtLeast?.("6.1") ?? true)
		? webApp.BackButton
		: undefined;
}

// Whether Telegram shows its native back button, for pages whose back action
// is registered by a parent.
export function hasTelegramBackButton(): boolean {
	return telegramBackButton() !== undefined;
}

export function useTelegramBackButton(onBack: () => void) {
	const backButton = telegramBackButton();
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
