import { useLocation } from "react-router";
import { useSessionNavigation } from "@/shared/navigation/SessionNavigation";
import { useCallback, useEffect, useRef, useState } from "react";
import type { Identity } from "@/api";
import { useIdentities } from "@/shared/lib/useIdentities";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";

export type HoldingsFilterKey = "platform" | "identityId" | "symbol";
export interface HoldingsFilterValues {
  platform: string;
  identityId: string;
  symbol: string;
}
type FilterIdentity = Pick<Identity, "id" | "platform" | "displayName" | "status">;

/** Reconcile configured candidates without silently assigning a Platform. */
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

/** URL-backed filters with a search draft saved before internal navigation. */
export function useHoldingsFilters(portfolioId: string) {
  const { params, update } = useSearchState();
  const location = useLocation();
  const navigation = useSessionNavigation();
  const identities = useIdentities(portfolioId);
  const platforms = usePlatforms();
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
  const legacy = params.has("accountId") || params.has("groupBy");
  useEffect(() => {
    if (legacy || needsCorrection) {
      update({
        platform: normalized.platform,
        identityId: normalized.identityId,
        accountId: "",
        groupBy: "",
      });
    }
  }, [legacy, needsCorrection, normalized.platform, normalized.identityId, update]);

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
      !legacy &&
      !needsCorrection &&
      ((!values.platform && !values.identityId) || Boolean(identities.data)),
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
