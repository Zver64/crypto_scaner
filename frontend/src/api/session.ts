import type { Session } from "@/api/generated/models";
import { readResponseData } from "@/api/response";
import { getTelegramInitData, telegramUserID } from "@/app/telegram";

// The token survives WebView reloads but not the end of the Mini App session.
const storageKey = "crypto-scanner.session";

/** A session token with the Telegram user it was issued for. */
interface StoredSession {
	telegramID: number;
	token: string;
}

let session: StoredSession | undefined = readStoredSession();
let pending: Promise<string | undefined> | undefined;

/**
 * Returns the session token, exchanging Telegram init data for one when none
 * is held. Resolves undefined outside Telegram and rejects with the API error
 * of a failed exchange.
 */
export function getSessionToken(): Promise<string | undefined> {
	// Telegram Web can reopen the app for another account in the same tab.
	if (session && session.telegramID !== telegramUserID()) {
		forgetSession();
	}
	if (session) return Promise.resolve(session.token);
	pending ??= exchangeInitData().finally(() => {
		pending = undefined;
	});
	return pending;
}

/** Forgets a token the backend rejected, unless a newer one replaced it. */
export function invalidateSessionToken(rejected: string): void {
	if (session?.token === rejected) forgetSession();
}

function forgetSession(): void {
	session = undefined;
	writeStoredSession(undefined);
}

/**
 * Sends a request with the session token and retries once with a new session
 * when the backend rejects the token.
 */
export async function fetchWithSession(
	url: string,
	options: RequestInit,
): Promise<Response> {
	const sent = await getSessionToken();
	const response = await fetch(url, withToken(options, sent));
	if (response.status !== 401 || !sent) return response;
	invalidateSessionToken(sent);
	const renewed = await getSessionToken();
	return renewed ? fetch(url, withToken(options, renewed)) : response;
}

function withToken(options: RequestInit, sessionToken: string | undefined) {
	if (!sessionToken) return options;
	const headers = new Headers(options.headers);
	headers.set("Authorization", `Bearer ${sessionToken}`);
	return { ...options, headers };
}

async function exchangeInitData(): Promise<string | undefined> {
	const initData = getTelegramInitData()?.trim();
	const telegramID = telegramUserID(initData);
	if (!initData || telegramID === undefined) return undefined;
	const response = await fetch("/api/v1/auth/session", {
		headers: { Authorization: `tma ${initData}` },
		method: "POST",
	});
	const data = await readResponseData(response);
	session = { telegramID, token: (data as Session).token };
	writeStoredSession(session);
	return session.token;
}

function readStoredSession(): StoredSession | undefined {
	try {
		const raw = window.sessionStorage.getItem(storageKey);
		if (!raw) return undefined;
		const stored = JSON.parse(raw) as Partial<StoredSession>;
		return typeof stored.token === "string" &&
			typeof stored.telegramID === "number"
			? { telegramID: stored.telegramID, token: stored.token }
			: undefined;
	} catch {
		return undefined;
	}
}

function writeStoredSession(value: StoredSession | undefined): void {
	try {
		if (value) window.sessionStorage.setItem(storageKey, JSON.stringify(value));
		else window.sessionStorage.removeItem(storageKey);
	} catch {
		// Storage may be unavailable; the session stays in memory.
	}
}
