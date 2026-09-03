import { createContext, useContext } from "react";
import type { BusinessRequestPermission } from "@/utils/business-request-permission";

export const BusinessRequestContext = createContext<
	BusinessRequestPermission | undefined
>(undefined);

export function useBusinessRequestPermission() {
	const permission = useContext(BusinessRequestContext);

	if (!permission) {
		throw new Error(
			"useBusinessRequestPermission must be used inside the application shell",
		);
	}

	return permission;
}
