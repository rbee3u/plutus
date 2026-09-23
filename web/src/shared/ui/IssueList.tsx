import type { Issue } from "@/api/generated/types.gen";
import styles from "@/features/pages.module.css";

/** Machine codes (issue codes, reason codes, error classes) in one mono style. */
export function Codes({ values }: { values: string[] }) {
  return (
    <>
      {values.map((value, index) => (
        <span key={value}>
          {index > 0 ? " " : ""}
          <code className={styles.issueCode}>{value}</code>
        </span>
      ))}
    </>
  );
}

/** A page section for issues: a heading and list when there are any, one quiet line otherwise. */
export function IssuesPanel({ issues, empty }: { issues: Issue[]; empty: string }) {
  if (issues.length === 0) {
    return <p className={styles.muted}>{empty}</p>;
  }
  return (
    <section className={styles.panel}>
      <h2>Issues</h2>
      <IssueList issues={issues} />
    </section>
  );
}

export function IssueList({
  issues,
  empty = "No issues for this view.",
}: {
  issues: Issue[];
  empty?: string;
}) {
  if (issues.length === 0) {
    return <p className={styles.muted}>{empty}</p>;
  }
  return (
    <ul className={styles.issues}>
      {issues.map((issue, index) => {
        const scope = [issue.accountName, issue.symbol].filter(Boolean).join(" · ");
        return (
          <li key={`${issue.code}-${issue.accountId ?? ""}-${issue.symbol ?? ""}-${String(index)}`}>
            <span>
              <Codes values={[issue.code]} /> {issue.detail}
            </span>
            {scope ? <span className={styles.issueMeta}>{scope}</span> : null}
          </li>
        );
      })}
    </ul>
  );
}
