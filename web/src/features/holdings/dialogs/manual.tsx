import { Pencil, Trash2 } from "lucide-react";
import { useId } from "react";
import { getManualBalances, keys } from "@/api";
import { useSessionScope } from "@/shared/lib/session";
import { usePortfolioQuery } from "@/shared/lib/usePortfolioQuery";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";

export interface ManualTarget {
  identityId: string;
  symbol?: string;
  action?: "edit" | "delete";
}

/** Each editor reads its own current ledger without background replacement of its draft. */
export function useManual(portfolioId: string, identityId: string) {
  const { userId } = useSessionScope();
  const editorId = useId();
  const key = keys.manualEditor(userId, portfolioId, identityId, editorId);
  const query = usePortfolioQuery({
    portfolioId,
    key: () => key,
    ready: Boolean(identityId),
    refetchOnMount: "always",
    refetchOnWindowFocus: false,
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
    <>
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
    </>
  );
}
