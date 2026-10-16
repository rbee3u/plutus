import { useLocation } from "react-router";
import { useSessionNavigation } from "@/shared/navigation/SessionNavigation";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Identity, Issue } from "@/api";
import { useIdentities } from "@/shared/lib/useIdentities";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { accountIssueScope } from "@/shared/lib/valuationRows";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";

export type HoldingsFilterKey = "platform" | "identityId" | "symbol";
export interface HoldingsFilterValues {
  platform: string;
  identityId: string;
  symbol: string;
}
type FilterIdentity = Pick<Identity, "id" | "platform" | "displayName" | "status">;

/** Reconcile configured candidates without silently assigning a platform. */
export function normalizeHoldingsSelection(
  values: HoldingsFilterValues,
  identities: FilterIdentity[],
): HoldingsFilterValues {
  const active = identities.filter((item) => item.status !== "archived");
  const identity = active.find((item) => item.id === values.identityId);
  const platformValid = active.some((item) => item.platform === values.platform);
  return {
    ...values,
    identityId: identity?.id ?? "",
    platform:
      platformValid && (!identity || identity.platform === values.platform) ? values.platform : "",
  };
}

export function holdingsCandidates(values: HoldingsFilterValues, identities: FilterIdentity[]) {
  const active = identities.filter((item) => item.status !== "archived");
  return {
    platforms: new Set(
      active
        .filter((item) => !values.identityId || item.id === values.identityId)
        .map((item) => item.platform),
    ),
    identities: active.filter((item) => !values.platform || item.platform === values.platform),
  };
}

/** List issues describe the portfolio, while empty states describe the filtered scope. */
export function holdingsScopeHasMissingReport(
  issues: Pick<Issue, "code" | "accountId">[],
  values: HoldingsFilterValues,
  identities: (Pick<Identity, "id" | "platform"> & { accounts: { id: string }[] })[],
) {
  const missing = issues.filter((issue) => issue.code === "missing");
  if (!values.platform && !values.identityId) return missing.length > 0;
  const accountIds = new Set(
    identities
      .filter(
        (identity) =>
          (!values.platform || identity.platform === values.platform) &&
          (!values.identityId || identity.id === values.identityId),
      )
      .flatMap((identity) => identity.accounts.map((account) => account.id)),
  );
  return missing.some((issue) => issue.accountId && accountIds.has(issue.accountId));
}

/** URL-backed filters with a search draft saved before internal navigation. */
export function useHoldingsFilters(portfolioId: string) {
  const { params, update } = useSearchState();
  const location = useLocation();
  const navigation = useSessionNavigation();
  const identities = useIdentities(portfolioId);
  const platforms = usePlatforms();
  const resolveIssueScope = useMemo(
    () =>
      accountIssueScope(
        identities.data?.items.flatMap((identity) =>
          identity.accounts.map((account) => ({
            accountId: account.id,
            accountName: account.displayName,
            identityName: identity.displayName,
            platform: identity.platform,
          })),
        ),
        platforms.data?.items,
      ),
    [identities.data, platforms.data],
  );
  const values: HoldingsFilterValues = {
    platform: params.get("platform") ?? "",
    identityId: params.get("identityId") ?? "",
    symbol: params.get("symbol") ?? "",
  };
  const normalized = identities.isSuccess
    ? normalizeHoldingsSelection(values, identities.data.items)
    : values;
  const needsCorrection =
    normalized.platform !== values.platform || normalized.identityId !== values.identityId;
  useEffect(() => {
    if (needsCorrection) {
      update({
        platform: normalized.platform,
        identityId: normalized.identityId,
      });
    }
  }, [needsCorrection, normalized.platform, normalized.identityId, update]);

  const [open, setOpen] = useState<HoldingsFilterKey | null>(null);
  const [draft, setDraft] = useState({ source: values.symbol, value: values.symbol });
  const pending = useRef<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const composing = useRef(false);
  const cancelTimer = useCallback(() => {
    if (timer.current !== null) clearTimeout(timer.current);
    timer.current = null;
  }, []);
  const flush = useCallback(() => {
    cancelTimer();
    if (pending.current !== null && !composing.current) {
      const symbol = pending.current.trim();
      // Keep the submitted draft until the URL acknowledges it. Pointer dismissal
      // and a tab click can otherwise race React Router's non-queued updates.
      update({ symbol });
    }
  }, [cancelTimer, update]);
  // Back navigation replaces a draft instead of letting its old timer overwrite the URL.
  useEffect(() => {
    pending.current = null;
    cancelTimer();
    return cancelTimer;
  }, [values.symbol, cancelTimer]);

  const search = params.toString();
  useEffect(
    () =>
      navigation.register({
        pathname: location.pathname,
        search: () => {
          cancelTimer();
          const next = new URLSearchParams(search);
          if (pending.current !== null) {
            const symbol = pending.current.trim();
            if (symbol) next.set("symbol", symbol);
            else next.delete("symbol");
            next.delete("offset");
          }
          pending.current = null;
          composing.current = false;
          setOpen(null);
          const value = next.toString();
          return value ? `?${value}` : "";
        },
      }),
    [cancelTimer, location.pathname, navigation, search],
  );

  function editSymbol(value: string) {
    setDraft({ source: values.symbol, value });
    pending.current = value;
    cancelTimer();
    if (!composing.current) {
      if (!value) flush();
      else timer.current = setTimeout(flush, 250);
    }
  }
  function change(next: Partial<HoldingsFilterValues>) {
    cancelTimer();
    const symbol = pending.current;
    pending.current = null;
    composing.current = false;
    update({ ...(symbol === null ? {} : { symbol: symbol.trim() }), ...next });
    if (next.symbol !== undefined) setDraft({ source: next.symbol, value: next.symbol });
    setOpen(null);
  }
  const close = useCallback(() => {
    flush();
    setOpen(null);
  }, [flush]);
  const candidates = holdingsCandidates(values, identities.data?.items ?? []);
  const symbolDraft = draft.source === values.symbol ? draft.value : values.symbol;
  return {
    values,
    symbolDraft,
    open,
    close,
    change,
    editSymbol,
    beginComposition: () => {
      composing.current = true;
      cancelTimer();
    },
    endComposition: (value: string) => {
      composing.current = false;
      editSymbol(value);
    },
    toggle: (key: HoldingsFilterKey) => {
      flush();
      setOpen((current) => (current === key ? null : key));
    },
    reset: () => change({ platform: "", identityId: "", symbol: "" }),
    hasFilters: Boolean(values.platform || values.identityId || values.symbol || symbolDraft),
    ready:
      !needsCorrection && ((!values.platform && !values.identityId) || Boolean(identities.data)),
    offset: readOffset(params),
    page: (offset: number) => {
      cancelTimer();
      const symbol = pending.current;
      pending.current = null;
      update({
        ...(symbol === null ? {} : { symbol: symbol.trim() }),
        offset: offset > 0 ? String(offset) : "",
      });
    },
    candidatesReady: Boolean(identities.data),
    resolveIssueScope,
    identities,
    platforms,
    platformOptions: [...candidates.platforms].map((id) => ({
      value: id,
      label: platformLabel(platforms.data?.items, id),
    })),
    identityOptions: candidates.identities.map((item) => ({
      value: item.id,
      label: item.displayName,
    })),
    platformName: platformLabel(platforms.data?.items, values.platform),
    identityName:
      identities.data?.items.find((item) => item.id === values.identityId)?.displayName ??
      "Loading…",
  };
}

export type HoldingsFilters = ReturnType<typeof useHoldingsFilters>;
