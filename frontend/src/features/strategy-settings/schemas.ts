import { z } from "zod";

// The id of a saved strategy or signal.
export const strategyIdSchema = z.uuid();

export const strategyNameSchema = z.string().trim().min(1, "Enter a name");
