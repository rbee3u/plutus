import type { Asset } from "@/api";
import type { CatalogFilters } from "../hooks/useCatalogFilters";

export const PAGE_SIZE = 50;

export function assetEtag(item: Asset): string {
  return `"asset:${String(item.mappingVersion)}"`;
}

export function exclusionEtag(item: Asset): string {
  return `"asset-exclusion:${String(item.exclusionVersion)}"`;
}

export interface MappingRequest {
  nonce: number;
  item: Asset;
}

export interface TabProps {
  portfolioId: string;
  filters: CatalogFilters;
  timezone?: string;
}
