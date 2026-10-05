import { plainNumberText } from "@/shared/lib/amount";
import styles from "@/shared/styles/page.module.css";

/** Read zero-count shorthand as the rounded ordinary decimal, without adding a control. */
export function NumericText({ children }: { children: string }) {
  const plainText = plainNumberText(children);
  if (plainText === children) return children;
  return (
    <span>
      <span aria-hidden="true">{children}</span>
      <span className={styles.srOnly}>{plainText}</span>
    </span>
  );
}
