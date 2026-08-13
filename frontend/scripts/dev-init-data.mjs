export function requireEnvironmentValue(values, name) {
	const value = values[name]?.trim();
	if (!value) {
		throw new Error(`${name} is required in the repository root .env file`);
	}
	return value;
}

export function parsePositiveInteger(value, name) {
	if (!/^\d+$/.test(value)) {
		throw new Error(`${name} must be a positive integer`);
	}
	const parsed = Number(value);
	if (!Number.isSafeInteger(parsed) || parsed <= 0) {
		throw new Error(`${name} must be a positive safe integer`);
	}
	return parsed;
}

export function upsertEnvironmentValue(contents, name, value) {
	const entry = `${name}=${value}`;
	const lines = contents.split(/\r?\n/);
	while (lines.at(-1) === "") {
		lines.pop();
	}
	const index = lines.findIndex((line) => line.startsWith(`${name}=`));
	if (index >= 0) {
		lines[index] = entry;
	} else {
		lines.push(entry);
	}

	return `${lines.join("\n")}\n`;
}

export function createDevelopmentInitData({
	botToken,
	now,
	signInitData,
	telegramID,
}) {
	return signInitData(
		{
			user: {
				id: telegramID,
				first_name: "Development",
				last_name: "User",
				language_code: "en",
			},
		},
		botToken,
		now(),
	);
}
