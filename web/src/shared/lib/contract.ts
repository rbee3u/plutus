import type { NullableDecimal } from "@/api";
import { formatDecimal } from "./amount";

export interface ContractSizeFacts {
  platform: string;
  kind: string;
  baseAssetSymbol?: string | null;
  quoteAssetSymbol?: string | null;
  contractSize: NullableDecimal;
}

/**
 * Contract size like the Catalog futures column: linear contracts use the base
 * quantity, inverse contracts the quote face amount. Binance linear quantities
 * are already base units, so an absent recorded size displays 1 with the base
 * symbol. Digit-leading base symbols use ×. Missing sizes stay unknown.
 */
export function formatContractSize(item: ContractSizeFacts): string {
  const isInverse = item.kind.includes("inverse");
  const unit = isInverse ? item.quoteAssetSymbol : item.baseAssetSymbol;
  const nativeBaseQuantity =
    item.platform === "binance" &&
    (item.kind === "linear_perpetual" || item.kind === "linear_delivery") &&
    Boolean(unit);
  const size = formatDecimal(item.contractSize ?? (nativeBaseQuantity ? "1" : null));
  const separator = !isInverse && unit && /^\d/.test(unit) ? " × " : " ";
  return size !== "—" && unit ? `${size}${separator}${unit}` : size;
}
