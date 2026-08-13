import { chmod, readFile, writeFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { sign } from "@tma.js/init-data-node";
import { loadEnv } from "vite";

const repositoryRoot = fileURLToPath(new URL("../..", import.meta.url));
const privateEnvPath = fileURLToPath(new URL("../../.env.local", import.meta.url));
const env = loadEnv("development", repositoryRoot, "");

const botToken = required(env, "TELEGRAM_BOT_TOKEN");
const maxAge = required(env, "TELEGRAM_INIT_DATA_MAX_AGE");
const telegramID = positiveInteger(
	required(env, "ADMIN_TELEGRAM_ID"),
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

function required(values, name) {
	const value = values[name]?.trim();
	if (!value) {
		throw new Error(`${name} is required in the repository root .env file`);
	}
	return value;
}

function positiveInteger(value, name) {
	if (!/^\d+$/.test(value)) {
		throw new Error(`${name} must be a positive integer`);
	}
	const parsed = Number(value);
	if (!Number.isSafeInteger(parsed) || parsed <= 0) {
		throw new Error(`${name} must be a positive safe integer`);
	}
	return parsed;
}

async function upsertEnvValue(path, name, value) {
	let contents = "";
	try {
		contents = await readFile(path, "utf8");
	} catch (error) {
		if (error?.code !== "ENOENT") {
			throw error;
		}
	}

	const entry = `${name}=${value}`;
	const lines = contents.split(/\r?\n/);
	const index = lines.findIndex((line) => line.startsWith(`${name}=`));
	if (index >= 0) {
		lines[index] = entry;
	} else {
		if (lines.length > 1 || lines[0] !== "") {
			lines.push(entry);
		} else {
			lines[0] = entry;
		}
	}

	const normalized = lines
		.filter((line, lineIndex) => line !== "" || lineIndex < lines.length - 1)
		.join("\n");
	await writeFile(path, `${normalized}\n`, { mode: 0o600 });
	await chmod(path, 0o600);
}
