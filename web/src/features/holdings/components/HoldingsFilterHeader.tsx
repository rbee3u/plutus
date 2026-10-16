import type { HoldingsFilterKey, HoldingsFilters } from "../hooks/useHoldingsFilters";
import type { DataColumn } from "@/shared/ui/DataTable";
import { TableFilterHeader } from "@/shared/ui/TableFilterHeader";

/** A column label, current condition, and accessible filter outside the table clip. */
export function HoldingsFilterHeader({
  label,
  filterKey,
  filters,
}: {
  label: string;
  filterKey: HoldingsFilterKey;
  filters: HoldingsFilters;
}) {
  const search = filterKey === "symbol";
  const value = search ? filters.symbolDraft : filters.values[filterKey];
  const display = !value
    ? ""
    : search
      ? value
      : filterKey === "platform"
        ? filters.platformName
        : filters.identityName;
  const options = [
    { value: "", label: "All" },
    ...(filterKey === "platform" ? filters.platformOptions : filters.identityOptions),
  ];

  return (
    <TableFilterHeader
      label={label}
      display={display}
      active={Boolean(value)}
      open={filters.open === filterKey}
      disabled={!search && !filters.candidatesReady}
      type={search ? "search" : "select"}
      onToggle={() => filters.toggle(filterKey)}
      onClose={filters.close}
      onClear={() => filters.change({ [filterKey]: "" })}
      options={options}
      selectedValue={value}
      onSelect={(val) => filters.change({ [filterKey]: val })}
      searchDraft={filters.symbolDraft}
      searchPlaceholder={label === "Pair" ? "Contains, e.g. BTC or USDT" : "Contains, e.g. BTC"}
      searchHelp={
        label === "Pair"
          ? "Search by asset symbol; partial matches are case-insensitive."
          : "Search supports partial matches and ignores case."
      }
      onSearchChange={filters.editSymbol}
      onCompositionStart={filters.beginComposition}
      onCompositionEnd={filters.endComposition}
    />
  );
}

/** Shared ownership columns for flat Holdings tables. */
export function holdingsAccountColumns(filters?: HoldingsFilters): DataColumn[] {
  return [
    {
      key: "platform",
      header: "Platform",
      nowrap: true,
      width: "140px",
      headerContent: filters ? (
        <HoldingsFilterHeader label="Platform" filterKey="platform" filters={filters} />
      ) : undefined,
    },
    {
      key: "identityName",
      header: "Identity",
      nowrap: true,
      width: "160px",
      headerContent: filters ? (
        <HoldingsFilterHeader label="Identity" filterKey="identityId" filters={filters} />
      ) : undefined,
    },
  ];
}
