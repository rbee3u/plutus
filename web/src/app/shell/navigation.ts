import {
  BarChart3,
  Briefcase,
  Database,
  Landmark,
  LayoutDashboard,
  ListOrdered,
  Scale,
  Users,
  Wallet,
  type LucideIcon,
} from "lucide-react";

export const LAST_PORTFOLIO_KEY = "plutus-last-portfolio-id";

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  phase: "working" | "2" | "3";
  activeSuffixes: readonly string[];
}

export interface NavGroup {
  id: string;
  label: string | null;
  items: NavItem[];
}

export interface ModuleSubview {
  label: string;
  title: string;
  pathSuffix: string;
  catalogTab?: "listings" | "instruments";
}

export interface ModuleDefinition {
  id: string;
  label: string;
  icon: LucideIcon;
  groupId: "overview" | "portfolio" | "manage";
  groupLabel: string | null;
  defaultPathSuffix: string;
  activePathSuffixes: readonly string[];
  subviews: readonly ModuleSubview[];
}

/** Target nine-module tree. Stage 1 does not render this. */
export const MODULE_DEFINITIONS: readonly ModuleDefinition[] = [
  {
    id: "overview",
    label: "Overview",
    icon: LayoutDashboard,
    groupId: "overview",
    groupLabel: null,
    defaultPathSuffix: "/overview",
    activePathSuffixes: ["/overview"],
    subviews: [{ label: "Overview", title: "Overview", pathSuffix: "/overview" }],
  },
  {
    id: "balances",
    label: "Balances",
    icon: Wallet,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/portfolio/balances",
    activePathSuffixes: ["/portfolio/balances"],
    subviews: [{ label: "Balances", title: "Balances", pathSuffix: "/portfolio/balances" }],
  },
  {
    id: "positions",
    label: "Positions",
    icon: Briefcase,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/portfolio/positions",
    activePathSuffixes: ["/portfolio/positions"],
    subviews: [{ label: "Positions", title: "Positions", pathSuffix: "/portfolio/positions" }],
  },
  {
    id: "exposure",
    label: "Exposure",
    icon: Scale,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/portfolio/exposure",
    activePathSuffixes: ["/portfolio/exposure"],
    subviews: [{ label: "Exposure", title: "Exposure", pathSuffix: "/portfolio/exposure" }],
  },
  {
    id: "history",
    label: "History",
    icon: BarChart3,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/history/valuations",
    activePathSuffixes: ["/history/valuations", "/history/value-changes"],
    subviews: [
      { label: "Valuations", title: "Valuations", pathSuffix: "/history/valuations" },
      { label: "Value Changes", title: "Value Changes", pathSuffix: "/history/value-changes" },
    ],
  },
  {
    id: "capital",
    label: "Capital",
    icon: Landmark,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/capital/baseline",
    activePathSuffixes: ["/capital/baseline", "/capital/flows"],
    subviews: [
      { label: "Baseline", title: "Baseline", pathSuffix: "/capital/baseline" },
      { label: "Flows", title: "Flows", pathSuffix: "/capital/flows" },
    ],
  },
  {
    id: "data-sources",
    label: "Data Sources",
    icon: Users,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/identities",
    activePathSuffixes: ["/data/identities", "/data/connections"],
    subviews: [
      { label: "Identities", title: "Identities", pathSuffix: "/data/identities" },
      { label: "Connections", title: "Connections", pathSuffix: "/data/connections" },
    ],
  },
  {
    id: "asset-catalog",
    label: "Asset Catalog",
    icon: Database,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/catalog",
    activePathSuffixes: ["/data/catalog"],
    subviews: [
      {
        label: "Asset Listings",
        title: "Asset Listings",
        pathSuffix: "/data/catalog",
        catalogTab: "listings",
      },
      {
        label: "Instruments",
        title: "Instruments",
        pathSuffix: "/data/catalog",
        catalogTab: "instruments",
      },
    ],
  },
  {
    id: "data-activity",
    label: "Data Activity",
    icon: ListOrdered,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/observations",
    activePathSuffixes: ["/data/observations", "/data/jobs", "/data/audit"],
    subviews: [
      { label: "Observations", title: "Observations", pathSuffix: "/data/observations" },
      { label: "Jobs", title: "Jobs", pathSuffix: "/data/jobs" },
      { label: "Audit Log", title: "Audit", pathSuffix: "/data/audit" },
    ],
  },
];

const PAGE_TITLES: readonly {
  suffix: string;
  title: string;
  catalogTab?: "listings" | "instruments";
}[] = [
  ...MODULE_DEFINITIONS.flatMap((module) =>
    module.subviews.map((view) => ({
      suffix: view.pathSuffix,
      title: view.title,
      catalogTab: view.catalogTab,
    })),
  ),
  { suffix: "/settings/portfolio", title: "Portfolio" },
];

export function isAccountPath(pathname: string): boolean {
  return pathname === "/account/security" || pathname.startsWith("/account/");
}

export function catalogTabFromSearch(search: string): "listings" | "instruments" {
  const query = search.startsWith("?") ? search.slice(1) : search;
  return new URLSearchParams(query).get("tab") === "instruments" ? "instruments" : "listings";
}

export function catalogTabHref(search: string, tab: "listings" | "instruments"): string {
  const query = search.startsWith("?") ? search.slice(1) : search;
  const next = new URLSearchParams(query);
  next.set("tab", tab);
  next.delete("offset");
  return `?${next.toString()}`;
}

export function portfolioPathSuffix(pathname: string): string | null {
  const match = pathname.match(/^\/p\/[^/]+(\/.*)$/);
  return match?.[1] ?? null;
}

export function isPathSuffixActive(pathname: string, suffixes: readonly string[]): boolean {
  const current = portfolioPathSuffix(pathname);
  return current !== null && suffixes.includes(current);
}

export function isNavItemActive(pathname: string, item: Pick<NavItem, "activeSuffixes">): boolean {
  return isPathSuffixActive(pathname, item.activeSuffixes);
}

export function activeModuleId(pathname: string): string | null {
  const current = portfolioPathSuffix(pathname);
  if (current === null) {
    return null;
  }
  return (
    MODULE_DEFINITIONS.find((module) => module.activePathSuffixes.includes(current))?.id ?? null
  );
}

export function pageTitle(pathname: string, search = ""): string {
  if (isAccountPath(pathname)) {
    return "Account & Security";
  }
  const suffix = portfolioPathSuffix(pathname);
  if (suffix === null) {
    return "Plutus";
  }
  const catalogTab = catalogTabFromSearch(search);
  return (
    PAGE_TITLES.find(
      (row) =>
        row.suffix === suffix && (row.catalogTab === undefined || row.catalogTab === catalogTab),
    )?.title ?? "Plutus"
  );
}

export function navigationFor(portfolioId: string): NavGroup[] {
  const base = `/p/${portfolioId}`;
  const groups: NavGroup[] = [];
  for (const definition of MODULE_DEFINITIONS) {
    let group = groups.find((item) => item.id === definition.groupId);
    if (!group) {
      group = { id: definition.groupId, label: definition.groupLabel, items: [] };
      groups.push(group);
    }
    group.items.push({
      to: `${base}${definition.defaultPathSuffix}`,
      label: definition.label,
      icon: definition.icon,
      phase: "working",
      activeSuffixes: definition.activePathSuffixes,
    });
  }
  return groups;
}
