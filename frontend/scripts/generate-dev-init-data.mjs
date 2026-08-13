import { chmod, readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { sign } from "@tma.js/init-data-node";
import { loadEnv } from "vite";
import {
	parsePositiveInteger,
	requireEnvironmentValue,
	upsertEnvironmentValue,
} from "./dev-init-data.mjs";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const privateEnvPath = fileURLToPath(new URL("../../.env.local", import.meta.url));
const env = loadEnv("development", repositoryRoot, "");

const botToken = requireEnvironmentValue(env, "TELEGRAM_BOT_TOKEN");
const maxAge = requireEnvironmentValue(env, "TELEGRAM_INIT_DATA_MAX_AGE");
const telegramID = parsePositiveInteger(
	requireEnvironmentValue(env, "ADMIN_TELEGRAM_ID"),
	"ADMIN_TELEGRAM_ID",
);
const authDate = new Date();
const initData = sign(
	{
		user: {
			id: telegramID,
			first_name: "Development",
			last_name: "User",
			language_code: "en",
		},
	},
	botToken,
	authDate,
);

await upsertEnvValue(privateEnvPath, "TELEGRAM_DEV_INIT_DATA", initData);

console.log(
	`Development init data generated with configured lifetime ${maxAge}.`,
);

async function upsertEnvValue(path, name, value) {
	let contents = "";
	try {
		contents = await readFile(path, "utf8");
	} catch (error) {
		if (error?.code !== "ENOENT") {
			throw error;
		}
	}

	await writeFile(path, upsertEnvironmentValue(contents, name, value), {
		mode: 0o600,
	});
	await chmod(path, 0o600);
}
