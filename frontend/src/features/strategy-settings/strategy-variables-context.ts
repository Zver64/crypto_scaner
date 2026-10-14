import { createContext } from "react";
import type { StrategyVariable } from "@/api/generated/models";

// The variables of one rule builder. Conditions read them from here, since
// the builder renders its condition component itself.
export const StrategyVariablesContext = createContext<
	readonly StrategyVariable[]
>([]);
