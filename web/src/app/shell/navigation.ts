import {
  BarChart3,
  Briefcase,
  Database,
  FileClock,
  GitCompare,
  History,
  Landmark,
  LayoutDashboard,
  Link2,
  ListOrdered,
  Scale,
  Settings2,
  Shield,
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
  { suffix: "/overview", title: "Overview" },
  { suffix: "/portfolio/balances", title: "Balances" },
  { suffix: "/portfolio/positions", title: "Positions" },
  { suffix: "/portfolio/exposure", title: "Exposure" },
  { suffix: "/history/valuations", title: "Valuations" },
  { suffix: "/history/value-changes", title: "Value Changes" },
  { suffix: "/capital/baseline", title: "Baseline" },
  { suffix: "/capital/flows", title: "Flows" },
  { suffix: "/data/identities", title: "Identities" },
  { suffix: "/data/connections", title: "Connections" },
  { suffix: "/data/catalog", title: "Catalog" },
  { suffix: "/data/observations", title: "Observations" },
  { suffix: "/data/jobs", title: "Jobs" },
  { suffix: "/data/audit", title: "Audit" },
  { suffix: "/settings/portfolio", title: "Portfolio" },
  { suffix: "/settings/security", title: "Security" },
];

export function catalogTabFromSearch(search: string): "listings" | "instruments" {
  const query = search.startsWith("?") ? search.slice(1) : search;
  return new URLSearchParams(query).get("tab") === "instruments" ? "instruments" : "listings";
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
  return [
    {
      id: "overview",
      label: null,
      items: [
        {
          to: `${base}/overview`,
          label: "Overview",
          icon: LayoutDashboard,
          phase: "working",
          activeSuffixes: ["/overview"],
        },
      ],
    },
    {
      id: "portfolio",
      label: "Portfolio",
      items: [
        {
          to: `${base}/portfolio/balances`,
          label: "Balances",
          icon: Wallet,
          phase: "working",
          activeSuffixes: ["/portfolio/balances"],
        },
        {
          to: `${base}/portfolio/positions`,
          label: "Positions",
          icon: Briefcase,
          phase: "working",
          activeSuffixes: ["/portfolio/positions"],
        },
        {
          to: `${base}/portfolio/exposure`,
          label: "Exposure",
          icon: Scale,
          phase: "working",
          activeSuffixes: ["/portfolio/exposure"],
        },
      ],
    },
    {
      id: "history",
      label: "History",
      items: [
        {
          to: `${base}/history/valuations`,
          label: "Valuations",
          icon: BarChart3,
          phase: "working",
          activeSuffixes: ["/history/valuations"],
        },
        {
          to: `${base}/history/value-changes`,
          label: "Value Changes",
          icon: GitCompare,
          phase: "working",
          activeSuffixes: ["/history/value-changes"],
        },
      ],
    },
    {
      id: "capital",
      label: "Capital",
      items: [
        {
          to: `${base}/capital/baseline`,
          label: "Baseline",
          icon: Landmark,
          phase: "working",
          activeSuffixes: ["/capital/baseline"],
        },
        {
          to: `${base}/capital/flows`,
          label: "Flows",
          icon: History,
          phase: "working",
          activeSuffixes: ["/capital/flows"],
        },
      ],
    },
    {
      id: "data",
      label: "Data",
      items: [
        {
          to: `${base}/data/identities`,
          label: "Identities",
          icon: Users,
          phase: "working",
          activeSuffixes: ["/data/identities"],
        },
        {
          to: `${base}/data/connections`,
          label: "Connections",
          icon: Link2,
          phase: "working",
          activeSuffixes: ["/data/connections"],
        },
        {
          to: `${base}/data/catalog`,
          label: "Catalog",
          icon: Database,
          phase: "working",
          activeSuffixes: ["/data/catalog"],
        },
        {
          to: `${base}/data/observations`,
          label: "Observations",
          icon: ListOrdered,
          phase: "working",
          activeSuffixes: ["/data/observations"],
        },
        {
          to: `${base}/data/jobs`,
          label: "Jobs",
          icon: ListOrdered,
          phase: "working",
          activeSuffixes: ["/data/jobs"],
        },
        {
          to: `${base}/data/audit`,
          label: "Audit",
          icon: FileClock,
          phase: "working",
          activeSuffixes: ["/data/audit"],
        },
      ],
    },
    {
      id: "settings",
      label: "Settings",
      items: [
        {
          to: `${base}/settings/portfolio`,
          label: "Portfolio",
          icon: Settings2,
          phase: "working",
          activeSuffixes: ["/settings/portfolio"],
        },
        {
          to: `${base}/settings/security`,
          label: "Security",
          icon: Shield,
          phase: "working",
          activeSuffixes: ["/settings/security"],
        },
      ],
    },
  ];
}
