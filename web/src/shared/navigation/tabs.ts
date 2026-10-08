function searchQuery(search: string): URLSearchParams {
  return new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
}

/** The `?tab=` value when it is one of `tabs`, otherwise the first tab. */
export function tabFromSearch(search: string, tabs: readonly string[]): string {
  const requested = searchQuery(search).get("tab");
  return tabs.find((tab) => tab === requested) ?? tabs[0] ?? "";
}

/** A search string that switches the tab and restarts paging. */
export function tabHref(search: string, tab: string): string {
  const next = searchQuery(search);
  next.set("tab", tab);
  next.delete("offset");
  next.delete("cursor");
  return `?${next.toString()}`;
}
