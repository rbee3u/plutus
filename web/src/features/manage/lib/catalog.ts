import type { Asset } from "@/api";
import type { CatalogFilters } from "../hooks/useCatalogFilters";

export const PAGE_SIZE = 50;

export interface MappingRequest {
  nonce: number;
  item: Asset;
}

export interface TabProps {
  portfolioId: string;
  filters: CatalogFilters;
  timezone?: string;
}
