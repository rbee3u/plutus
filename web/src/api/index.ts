/**
 * The single data seam for the frontend. Features import generated SDK calls,
 * their wire types, unwrapping helpers, and query keys from here rather than
 * reaching into `@/api/generated` directly.
 */
export * from "./generated/sdk.gen";
export type * from "./generated/types.gen";
export * from "./client";
export * from "./keys";
