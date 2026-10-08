import type { ExpressionNode } from "@react-querybuilder/expr";

export interface PriceSetup {
	// Undefined when the price is disabled or a stored formula is unsupported.
	node?: ExpressionNode;
	// Preserve unsupported stored formulas in the manual editor.
	custom?: string;
	// Keep the stored source unchanged until the builder is edited.
	original?: string;
}
