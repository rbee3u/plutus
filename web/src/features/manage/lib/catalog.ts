import type { PlatformAsset } from "@/api";
import type { CatalogFilters } from "../hooks/useCatalogFilters";

export const PAGE_SIZE = 50;

export function platformAssetEtag(item: PlatformAsset): string {
  return `"platform-asset:${String(item.version)}"`;
}

export function exclusionEtag(item: PlatformAsset): string {
  return `"platform-asset-exclusion:${String(item.exclusionVersion)}"`;
}

export interface MappingRequest {
  nonce: number;
  item: PlatformAsset;
}

export interface TabProps {
  portfolioId: string;
  filters: CatalogFilters;
  timezone?: string;
}
