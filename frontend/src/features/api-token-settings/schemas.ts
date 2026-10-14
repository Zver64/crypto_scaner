import { z } from "zod";

export const tokenNameSchema = z.string().trim().min(1, "Enter a token name");
