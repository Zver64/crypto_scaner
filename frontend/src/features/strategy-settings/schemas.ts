import { z } from "zod";

export const strategyNameSchema = z.string().trim().min(1, "Enter a name");
