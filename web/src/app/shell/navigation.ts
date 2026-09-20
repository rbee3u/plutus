import {
  BarChart3,
  Briefcase,
  Database,
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

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  phase: "working" | "2" | "3";
}

export interface NavGroup {
  id: string;
  label: string | null;
  items: NavItem[];
}

export function navigationFor(portfolioId: string): NavGroup[] {
  const base = `/p/${portfolioId}`;
  return [
    {
      id: "overview",
      label: null,
      items: [
        { to: `${base}/overview`, label: "Overview", icon: LayoutDashboard, phase: "working" },
      ],
    },
    {
      id: "portfolio",
      label: "Portfolio",
      items: [
        { to: `${base}/portfolio/balances`, label: "Balances", icon: Wallet, phase: "working" },
        {
          to: `${base}/portfolio/positions`,
          label: "Positions",
          icon: Briefcase,
          phase: "working",
        },
        { to: `${base}/portfolio/exposure`, label: "Exposure", icon: Scale, phase: "3" },
      ],
    },
    {
      id: "history",
      label: "History",
      items: [
        { to: `${base}/history/valuations`, label: "Valuations", icon: BarChart3, phase: "3" },
        {
          to: `${base}/history/value-changes`,
          label: "Value Changes",
          icon: GitCompare,
          phase: "3",
        },
      ],
    },
    {
      id: "capital",
      label: "Capital",
      items: [
        { to: `${base}/capital/baseline`, label: "Baseline", icon: Landmark, phase: "working" },
        { to: `${base}/capital/flows`, label: "Flows", icon: History, phase: "working" },
      ],
    },
    {
      id: "data",
      label: "Data",
      items: [
        { to: `${base}/data/identities`, label: "Identities", icon: Users, phase: "3" },
        { to: `${base}/data/connections`, label: "Connections", icon: Link2, phase: "3" },
        { to: `${base}/data/catalog`, label: "Catalog", icon: Database, phase: "3" },
        { to: `${base}/data/observations`, label: "Observations", icon: ListOrdered, phase: "3" },
        { to: `${base}/data/jobs`, label: "Jobs", icon: ListOrdered, phase: "3" },
      ],
    },
    {
      id: "settings",
      label: "Settings",
      items: [
        { to: `${base}/settings/portfolio`, label: "Portfolio", icon: Settings2, phase: "working" },
        { to: `${base}/settings/security`, label: "Security", icon: Shield, phase: "working" },
      ],
    },
  ];
}

export function pageTitle(pathname: string): string {
  const groups = navigationFor("_");
  for (const group of groups) {
    for (const item of group.items) {
      const suffix = item.to.slice("/p/_".length);
      if (pathname.endsWith(suffix)) return item.label;
    }
  }
  return "Plutus";
}

export const LAST_PORTFOLIO_KEY = "plutus-last-portfolio-id";
