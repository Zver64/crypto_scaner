import { afterEach, expect, it, vi } from "vitest";
import {
	initializeTelegramMiniApp,
	openTelegramExternalLink,
	telegramRequestOptions,
} from "@/app/telegram";

afterEach(() => {
	vi.unstubAllGlobals();
});

it("does not initialize the same Telegram Mini App twice", () => {
	const webApp = {
		disableVerticalSwipes: vi.fn(),
		expand: vi.fn(),
		initData: "",
		isVersionAtLeast: vi.fn(() => true),
		ready: vi.fn(),
	};

	initializeTelegramMiniApp(webApp);
	initializeTelegramMiniApp(webApp);

	expect(webApp.disableVerticalSwipes).toHaveBeenCalledOnce();
});

it("provides Telegram authorization to generated request options", () => {
	vi.stubGlobal("window", {
		Telegram: { WebApp: { initData: " signed-init-data " } },
	});

	expect(telegramRequestOptions()).toEqual({
		headers: { Authorization: "tma signed-init-data" },
	});
});

it("omits authorization when Telegram init data is unavailable", () => {
	vi.stubGlobal("window", {});

	expect(telegramRequestOptions()).toBeUndefined();
});

it("opens external links with the Telegram client when available", () => {
	const openLink = vi.fn();
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
