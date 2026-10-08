import { tabFromSearch } from "@/shared/navigation/tabs";

export const CATALOG_TABS = ["listings", "instruments"] as const;
export type CatalogTab = (typeof CATALOG_TABS)[number];

export const ACTIVITY_TABS = ["cycles", "jobs", "audit"] as const;
export type ActivityTab = (typeof ACTIVITY_TABS)[number];

export function catalogTabFromSearch(search: string): CatalogTab {
  return tabFromSearch(search, CATALOG_TABS) === "instruments" ? "instruments" : "listings";
}

export function activityTabFromSearch(search: string): ActivityTab {
  const tab = tabFromSearch(search, ACTIVITY_TABS);
  return tab === "jobs" || tab === "audit" ? tab : "cycles";
}
