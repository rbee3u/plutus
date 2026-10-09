import type { AssetListing } from "@/api";
import type { CatalogFilters } from "../hooks/useCatalogFilters";

export const PAGE_SIZE = 50;

export function listingEtag(item: AssetListing): string {
  return `"listing:${String(item.version)}"`;
}

export function exclusionEtag(item: AssetListing): string {
  return `"listing-exclusion:${String(item.exclusionVersion)}"`;
}

export interface MappingRequest {
  nonce: number;
  item: AssetListing;
}

export interface TabProps {
  portfolioId: string;
  filters: CatalogFilters;
  timezone?: string;
}
