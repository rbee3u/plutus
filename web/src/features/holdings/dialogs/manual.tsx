import { Pencil, Trash2 } from "lucide-react";
import { useId } from "react";
import { getManualBalances, keys } from "@/api";
import type { ManualBalanceState } from "@/api";
import { parseDecimal } from "@/shared/lib/amount";
import { useSessionScope } from "@/shared/lib/session";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import shared from "@/shared/styles/page.module.css";

export interface ManualTarget {
  identityId: string;
  symbol?: string;
  action?: "edit" | "delete";
}

/** The editable manual records of one Identity; `fresh` reads them anew for the editor. */
export function useManual(portfolioId: string, identityId: string, fresh = false) {
  const { userId } = useSessionScope();
  const editorId = useId();
  const key = fresh
    ? keys.manualEditor(userId, portfolioId, identityId, editorId)
    : keys.manualBalances(userId, portfolioId, identityId);
  const query = usePortfolioQuery({
    portfolioId,
    key: () => key,
    ready: Boolean(identityId),
    refetchOnMount: fresh ? "always" : true,
    refetchOnWindowFocus: !fresh,
    load: (portfolioId) => getManualBalances({ path: { portfolioId, identityId } }),
  });
  return { ...query, key };
}

export function ManualActions({
  identityId,
  symbol,
  onOpen,
}: {
  identityId: string;
  symbol: string;
  onOpen: (target: ManualTarget) => void;
}) {
  return (
    <div className={shared.actions}>
      <IconButton
        label={`Edit manual ${symbol}`}
        onClick={() => onOpen({ identityId, symbol, action: "edit" })}
      >
        <Pencil size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <IconButton
        label={`Delete manual ${symbol}`}
        tone="dangerQuiet"
        onClick={() => onOpen({ identityId, symbol, action: "delete" })}
      >
        <Trash2 size={iconSize.sm} aria-hidden="true" />
      </IconButton>
    </div>
  );
}

/** Mutable changes are displayed separately from immutable valuation rows. */
export function manualChanges(state: ManualBalanceState) {
  const before = new Map(state.appliedItems.map((item) => [item.symbol, item.quantity]));
  const after = new Map(state.items.map((item) => [item.symbol, item.quantity]));
  return [...new Set([...before.keys(), ...after.keys()])].sort().flatMap((symbol) => {
    const oldQuantity = before.get(symbol) ?? "0";
    const quantity = after.get(symbol) ?? "0";
    return parseDecimal(oldQuantity)?.equals(parseDecimal(quantity) ?? "0")
      ? []
      : [{ symbol, oldQuantity, quantity }];
  });
}
