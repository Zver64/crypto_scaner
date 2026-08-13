export interface BusinessRequestPermission {
	allowed: boolean;
	authenticated: boolean;
	backendReady: boolean;
}

interface BusinessRequestPermissionInput {
	backendReady: boolean;
	isProduction: boolean;
	telegramInitData: string | undefined;
}

export function getBusinessRequestPermission({
	backendReady,
	isProduction,
	telegramInitData,
}: BusinessRequestPermissionInput): BusinessRequestPermission {
	const authenticated = !isProduction || Boolean(telegramInitData?.trim());

	return {
		allowed: authenticated && backendReady,
		authenticated,
		backendReady,
	};
}
