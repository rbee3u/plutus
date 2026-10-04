import {
  Activity,
  BarChart3,
  Database,
  Landmark,
  LayoutDashboard,
  Layers,
  Settings,
  Users,
  type LucideIcon,
} from "lucide-react";

export const LAST_PORTFOLIO_KEY = "plutus-last-portfolio-id";

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  activeSuffixes: readonly string[];
}

export interface NavGroup {
  id: string;
  label: string | null;
  items: NavItem[];
}

export interface ModuleSubview {
  label: string;
  pathSuffix: string;
  /** Subviews that share a path and switch with `?tab=`. */
  tab?: string;
}

export interface ModuleDefinition {
  id: string;
  label: string;
  icon: LucideIcon;
  groupId: "overview" | "portfolio" | "manage";
  groupLabel: string | null;
  defaultPathSuffix: string;
  subviews: readonly ModuleSubview[];
}

export const CATALOG_TABS = ["listings", "instruments"] as const;
export type CatalogTab = (typeof CATALOG_TABS)[number];

export const ACTIVITY_TABS = ["cycles", "jobs", "audit"] as const;
export type ActivityTab = (typeof ACTIVITY_TABS)[number];

export const MODULE_DEFINITIONS: readonly ModuleDefinition[] = [
  {
    id: "overview",
    label: "Overview",
    icon: LayoutDashboard,
    groupId: "overview",
    groupLabel: null,
    defaultPathSuffix: "/overview",
    subviews: [{ label: "Overview", pathSuffix: "/overview" }],
  },
  {
    id: "holdings",
    label: "Holdings",
    icon: Layers,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/holdings/balances",
    subviews: [
      { label: "Balances", pathSuffix: "/holdings/balances" },
      { label: "Futures", pathSuffix: "/holdings/futures" },
      { label: "Dual Investment", pathSuffix: "/holdings/dual-investment" },
    ],
  },
  {
    id: "history",
    label: "History",
    icon: BarChart3,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/history",
    subviews: [
      { label: "Valuation Points", pathSuffix: "/history" },
      { label: "Value Changes", pathSuffix: "/history/value-changes" },
    ],
  },
  {
    id: "capital",
    label: "Capital",
    icon: Landmark,
    groupId: "portfolio",
    groupLabel: "Portfolio",
    defaultPathSuffix: "/capital",
    subviews: [{ label: "Capital", pathSuffix: "/capital" }],
  },
  {
    id: "identities",
    label: "Identities",
    icon: Users,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/identities",
    subviews: [{ label: "Identities", pathSuffix: "/data/identities" }],
  },
  {
    id: "catalog",
    label: "Catalog",
    icon: Database,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/catalog",
    subviews: [
      { label: "Asset Listings", pathSuffix: "/data/catalog", tab: "listings" },
      { label: "Instruments", pathSuffix: "/data/catalog", tab: "instruments" },
    ],
  },
  {
    id: "activity",
    label: "Activity",
    icon: Activity,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/data/activity",
    subviews: [
      { label: "Collection Cycles", pathSuffix: "/data/activity", tab: "cycles" },
      { label: "Jobs", pathSuffix: "/data/activity", tab: "jobs" },
      { label: "Audit", pathSuffix: "/data/activity", tab: "audit" },
    ],
  },
  {
    id: "settings",
    label: "Settings",
    icon: Settings,
    groupId: "manage",
    groupLabel: "Manage",
    defaultPathSuffix: "/settings/portfolio",
    subviews: [{ label: "Portfolio settings", pathSuffix: "/settings/portfolio" }],
  },
];

export function isAccountPath(pathname: string): boolean {
  return pathname.startsWith("/account/");
}

function searchQuery(search: string): URLSearchParams {
  return new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
}

/** The `?tab=` value when it is one of `tabs`, otherwise the first tab. */
export function tabFromSearch(search: string, tabs: readonly string[]): string {
  const requested = searchQuery(search).get("tab");
  return tabs.find((tab) => tab === requested) ?? tabs[0] ?? "";
}

/** A search string that switches the tab and restarts paging. */
export function tabHref(search: string, tab: string): string {
  const next = searchQuery(search);
  next.set("tab", tab);
  next.delete("offset");
  next.delete("cursor");
  return `?${next.toString()}`;
}

export function catalogTabFromSearch(search: string): CatalogTab {
  return tabFromSearch(search, CATALOG_TABS) === "instruments" ? "instruments" : "listings";
}

export function activityTabFromSearch(search: string): ActivityTab {
  const tab = tabFromSearch(search, ACTIVITY_TABS);
  return tab === "jobs" || tab === "audit" ? tab : "cycles";
}

function portfolioRoute(pathname: string): { portfolioId: string; suffix: string } | null {
  const match = pathname.match(/^\/p\/([^/]+)(\/.*)$/);
  return match?.[1] && match[2] ? { portfolioId: match[1], suffix: match[2] } : null;
}

export function isNavItemActive(pathname: string, item: Pick<NavItem, "activeSuffixes">): boolean {
  const route = portfolioRoute(pathname);
  return route !== null && item.activeSuffixes.includes(route.suffix);
}

export interface PageTab {
  label: string;
  to: string;
  active: boolean;
}

export interface PageHeading {
  /** The top bar title: the module when it has tabs, otherwise the view. */
  title: string;
  /** The browser tab title, naming the active view. */
  documentTitle: string;
  tabs: PageTab[];
}

export function pageHeading(pathname: string, search = ""): PageHeading {
  if (isAccountPath(pathname)) {
    return { title: "Account & Security", documentTitle: "Account & Security", tabs: [] };
  }
  const route = portfolioRoute(pathname);
  const module = route
    ? MODULE_DEFINITIONS.find((item) =>
        item.subviews.some((view) => view.pathSuffix === route.suffix),
      )
    : undefined;
  if (!route || !module) {
    return { title: "Plutus", documentTitle: "Plutus", tabs: [] };
  }
  if (module.subviews.length < 2) {
    const label = module.subviews[0]?.label ?? module.label;
    return { title: label, documentTitle: label, tabs: [] };
  }
  const tabs = module.subviews.flatMap((view) => (view.tab ? [view.tab] : []));
  const currentTab = tabs.length > 0 ? tabFromSearch(search, tabs) : "";
  const items = module.subviews.map((view) => {
    const base = `/p/${route.portfolioId}${view.pathSuffix}`;
    const samePath = view.pathSuffix === route.suffix;
    return {
      label: view.label,
      to: view.tab ? `${base}${tabHref(search, view.tab)}` : base,
      active: view.tab ? samePath && currentTab === view.tab : samePath,
    };
  });
  const active = items.find((item) => item.active);
  return {
    title: module.label,
    documentTitle: active ? `${active.label} · ${module.label}` : module.label,
    tabs: items,
  };
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
      activeSuffixes: [...new Set(definition.subviews.map((view) => view.pathSuffix))],
    });
  }
  return groups;
}
