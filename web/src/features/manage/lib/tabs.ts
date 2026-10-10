import { tabFromSearch } from "@/shared/navigation/tabs";

export const CATALOG_TABS = ["assets", "futures"] as const;
export type CatalogTab = (typeof CATALOG_TABS)[number];

export const ACTIVITY_TABS = ["cycles", "audit"] as const;
export type ActivityTab = (typeof ACTIVITY_TABS)[number];

export function catalogTabFromSearch(search: string): CatalogTab {
  return tabFromSearch(search, CATALOG_TABS) === "futures" ? "futures" : "assets";
}

export function activityTabFromSearch(search: string): ActivityTab {
  const tab = tabFromSearch(search, ACTIVITY_TABS);
  return tab === "audit" ? tab : "cycles";
}
