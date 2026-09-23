import { useCallback } from "react";
import { useSearchParams } from "react-router";

/**
 * URL search params as page state. `update` merges keys, drops empty values,
 * and resets `offset` unless the update sets it, so a filter change always
 * starts from the first page.
 */
export function useSearchState() {
  const [params, setParams] = useSearchParams();
  const update = useCallback(
    (next: Record<string, string>) => {
      setParams(
        (current) => {
          const merged = new URLSearchParams(current);
          for (const [key, value] of Object.entries(next)) {
            if (value) {
              merged.set(key, value);
            } else {
              merged.delete(key);
            }
          }
          if (!("offset" in next)) {
            merged.delete("offset");
          }
          return merged;
        },
        { replace: true },
      );
    },
    [setParams],
  );
  return { params, update };
}

export function readOffset(params: URLSearchParams): number {
  return Math.max(0, Number(params.get("offset") ?? "0") || 0);
}

export function readChoice<T extends string>(
  value: string | null,
  options: readonly T[],
  fallback: T,
): T {
  return options.find((option) => option === value) ?? fallback;
}
