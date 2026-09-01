import { expect, it, vi } from "vitest";
import { initializeTelegramMiniApp } from "./telegram";

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
