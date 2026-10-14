import type { ReactNode } from "react";
import { plainNumberText } from "@/shared/lib/amount";
import styles from "@/shared/styles/page.module.css";

const SUBSCRIPT_DIGITS = "₀₁₂₃₄₅₆₇₈₉";
const SHORTHAND_PATTERN = /0\.0([₀₁₂₃₄₅₆₇₈₉]+)/g;

function renderVisualContent(text: string): ReactNode {
  const parts: ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;

  while ((match = SHORTHAND_PATTERN.exec(text)) !== null) {
    if (match.index > lastIndex) {
      parts.push(text.slice(lastIndex, match.index));
    }
    const count = [...match[1]]
      .map((ch) => SUBSCRIPT_DIGITS.indexOf(ch))
      .filter((idx) => idx !== -1)
      .join("");
    parts.push(
      "0.0",
      <sub key={match.index} className={styles.zeroCount}>
        {count}
      </sub>,
    );
    lastIndex = match.index + match[0].length;
  }

  if (lastIndex < text.length) {
    parts.push(text.slice(lastIndex));
  }

  return parts;
}

/** Read zero-count shorthand as the rounded ordinary decimal, without adding a control. */
export function NumericText({ children }: { children: string }) {
  const plainText = plainNumberText(children);
  if (plainText === children) return children;
  return (
    <span>
      <span aria-hidden="true">{renderVisualContent(children)}</span>
      <span className={styles.srOnly}>{plainText}</span>
    </span>
  );
}
