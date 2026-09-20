import { useLocation } from "react-router";
import { pageTitle } from "@/app/shell/navigation";
import styles from "@/features/pages.module.css";

export function PlaceholderPage() {
  const location = useLocation();
  const title = pageTitle(location.pathname);
  return (
    <section className={styles.page}>
      <h2>{title}</h2>
      <p className={styles.notice}>This route is reserved for a later implementation phase.</p>
    </section>
  );
}
