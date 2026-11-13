import { createContext } from "react";

import type { StrategyVariables } from "@/features/strategy-settings/types";

// What the operands of one rule builder read. Conditions read it from here,
// since the builder renders its condition component itself.
export const StrategyVariablesContext = createContext<StrategyVariables>({
	intervals: [],
	variables: [],
});
