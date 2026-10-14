import { afterEach, expect, it, vi } from "vitest";

import {
	initializeTelegramMiniApp,
	openTelegramExternalLink,
} from "@/app/telegram";

afterEach(() => {
	vi.unstubAllGlobals();
});

it("does not initialize the same Telegram Mini App twice", () => {
	const webApp = {
		disableVerticalSwipes: vi.fn<() => void>(),
		expand: vi.fn<() => void>(),
		initData: "",
		isVersionAtLeast: vi.fn<(version: string) => boolean>(() => true),
		ready: vi.fn<() => void>(),
	};

	initializeTelegramMiniApp(webApp);
	initializeTelegramMiniApp(webApp);

	expect(webApp.disableVerticalSwipes).toHaveBeenCalledOnce();
});

it("opens external links with the Telegram client when available", () => {
	const openLink = vi.fn<(url: string) => void>();
	vi.stubGlobal("window", { Telegram: { WebApp: { openLink } } });

	expect(
		openTelegramExternalLink("https://www.binance.com/en/trade/BTC_USDT"),
	).toBe(true);
	expect(openLink).toHaveBeenCalledWith(
		"https://www.binance.com/en/trade/BTC_USDT",
	);
});

it("falls back when the Telegram client is unavailable", () => {
	vi.stubGlobal("window", {});

	expect(
		openTelegramExternalLink("https://www.binance.com/en/trade/BTC_USDT"),
	).toBe(false);
});
