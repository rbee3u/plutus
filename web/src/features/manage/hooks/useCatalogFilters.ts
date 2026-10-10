import { useCallback, useEffect, useRef, useState } from "react";
import { useLocation } from "react-router";
import { platformLabel, usePlatforms } from "@/shared/lib/usePlatforms";
import { readOffset, useSearchState } from "@/shared/lib/searchParams";
import { useSessionNavigation } from "@/shared/navigation/SessionNavigation";

export type CatalogFilterKey = "platform" | "q" | "treatment" | "status";

export interface CatalogFilterValues {
  platform: string;
  q: string;
  excludedOnly: boolean;
}

export function useCatalogFilters() {
  const { params, update } = useSearchState();
  const location = useLocation();
  const navigation = useSessionNavigation();
  const platforms = usePlatforms();

  const values: CatalogFilterValues = {
    platform: params.get("platform") ?? "",
    q: params.get("q") ?? "",
    excludedOnly: params.get("excludedOnly") === "true",
  };

  const [open, setOpen] = useState<CatalogFilterKey | null>(null);
  const [draft, setDraft] = useState({ source: values.q, value: values.q });
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
      const q = pending.current.trim();
      update({ q, offset: "" });
    }
  }, [cancelTimer, update]);

  // Back navigation replaces a draft instead of letting its old timer overwrite the URL.
  useEffect(() => {
    pending.current = null;
    cancelTimer();
    return cancelTimer;
  }, [values.q, cancelTimer]);

  const search = params.toString();
  useEffect(
    () =>
      navigation.register({
        pathname: location.pathname,
        search: () => {
          cancelTimer();
          const next = new URLSearchParams(search);
          if (pending.current !== null) {
            const q = pending.current.trim();
            if (q) next.set("q", q);
            else next.delete("q");
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

  function editQuery(value: string) {
    setDraft({ source: values.q, value });
    pending.current = value;
    cancelTimer();
    if (!composing.current) {
      if (!value) flush();
      else timer.current = setTimeout(flush, 250);
    }
  }

  function change(next: { platform?: string; q?: string; excludedOnly?: boolean }) {
    cancelTimer();
    const q = pending.current;
    pending.current = null;
    composing.current = false;
    const updates: Record<string, string> = {
      offset: "",
      ...(q === null ? {} : { q: q.trim() }),
    };
    if (next.platform !== undefined) updates.platform = next.platform;
    if (next.q !== undefined) {
      updates.q = next.q.trim();
      setDraft({ source: next.q, value: next.q });
    }
    if (next.excludedOnly !== undefined) {
      updates.excludedOnly = next.excludedOnly ? "true" : "";
    }
    update(updates);
    setOpen(null);
  }

  const close = useCallback(() => {
    flush();
    setOpen(null);
  }, [flush]);

  const qDraft = draft.source === values.q ? draft.value : values.q;

  const platformOptions = [
    { value: "", label: "All" },
    ...(platforms.data?.items ?? []).map((item) => ({
      value: item.id,
      label: item.name,
    })),
  ];

  const statusOptions = [
    { value: "", label: "All" },
    { value: "true", label: "Excluded only" },
  ];

  const treatmentOptions = statusOptions;

  return {
    values,
    qDraft,
    open,
    close,
    change,
    editQuery,
    beginComposition: () => {
      composing.current = true;
      cancelTimer();
    },
    endComposition: (value: string) => {
      composing.current = false;
      editQuery(value);
    },
    toggle: (key: CatalogFilterKey) => {
      flush();
      setOpen((current) => (current === key ? null : key));
    },
    reset: () => {
      cancelTimer();
      pending.current = null;
      composing.current = false;
      setDraft({ source: "", value: "" });
      update({ platform: "", q: "", excludedOnly: "", offset: "" });
      setOpen(null);
    },
    hasFilters: Boolean(values.platform || values.q || values.excludedOnly || qDraft),
    offset: readOffset(params),
    page: (offset: number) => {
      cancelTimer();
      const q = pending.current;
      pending.current = null;
      update({
        ...(q === null ? {} : { q: q.trim() }),
        offset: offset > 0 ? String(offset) : "",
      });
    },
    platforms,
    platformOptions,
    platformName: platformLabel(platforms.data?.items, values.platform),
    statusOptions,
    statusName: values.excludedOnly ? "Excluded" : "All",
    treatmentOptions,
    treatmentName: values.excludedOnly ? "Excluded" : "All",
  };
}

export type CatalogFilters = ReturnType<typeof useCatalogFilters>;
