import { Link, Outlet, useLocation, useParams } from "react-router";
import {
  MODULE_DEFINITIONS,
  catalogTabFromSearch,
  catalogTabHref,
  isPathSuffixActive,
} from "@/app/shell/navigation";
import styles from "@/app/shell/Shell.module.css";

export interface SubnavItem {
  to: string;
  label: string;
  isActive: boolean;
}

export function RoutedSubnav({
  ariaLabel,
  items,
}: {
  ariaLabel: string;
  items: readonly SubnavItem[];
}) {
  return (
    <nav className={styles.subnav} aria-label={ariaLabel}>
      {items.map((item) => (
        <Link key={item.label} to={item.to} aria-current={item.isActive ? "page" : undefined}>
          {item.label}
        </Link>
      ))}
    </nav>
  );
}

export function ModuleSubnav({ moduleId }: { moduleId: string }) {
  const { portfolioId = "" } = useParams();
  const location = useLocation();
  const module = MODULE_DEFINITIONS.find((item) => item.id === moduleId);
  if (!module || module.subviews.length < 2) {
    return <Outlet />;
  }
  const catalogTab = catalogTabFromSearch(location.search);
  const items = module.subviews.map((view) => {
    if (view.catalogTab) {
      return {
        to: `${location.pathname}${catalogTabHref(location.search, view.catalogTab)}`,
        label: view.label,
        isActive: catalogTab === view.catalogTab,
      };
    }
    return {
      to: `/p/${portfolioId}${view.pathSuffix}`,
      label: view.label,
      isActive: isPathSuffixActive(location.pathname, [view.pathSuffix]),
    };
  });
  return (
    <>
      <RoutedSubnav ariaLabel={module.label} items={items} />
      <Outlet />
    </>
  );
}
