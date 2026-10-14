import { tabFromSearch } from "@/shared/navigation/tabs";

export const CAPITAL_TABS = ["flows", "events"] as const;
export type CapitalTab = (typeof CAPITAL_TABS)[number];

export function capitalTabFromSearch(search: string): CapitalTab {
  return tabFromSearch(search, CAPITAL_TABS) === "events" ? "events" : "flows";
}
