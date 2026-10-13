import { useCallback, useState } from "react";

/** A stack of visited cursors; the last entry is the page currently shown. */
export function useCursorPages() {
  const [cursors, setCursors] = useState<string[]>([""]);
  const cursor = cursors[cursors.length - 1] ?? "";
  const reset = useCallback(() => setCursors([""]), []);
  return {
    cursor,
    canPrev: cursors.length > 1,
    prev: () => setCursors((items) => (items.length > 1 ? items.slice(0, -1) : items)),
    next: (nextCursor: string | null | undefined) => {
      if (nextCursor) setCursors((items) => [...items, nextCursor]);
    },
    reset,
  };
}
