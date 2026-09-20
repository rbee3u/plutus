import type { ProjectionIssue } from "@/api/generated/types.gen";
import styles from "@/features/pages.module.css";

export function IssueList({ issues }: { issues: ProjectionIssue[] }) {
  if (issues.length === 0) {
    return <p className={styles.muted}>No projection issues for this view.</p>;
  }
  return (
    <ul className={styles.issues}>
      {issues.map((issue) => (
        <li key={issue.id ?? `${issue.code}-${issue.projectionKind}-${issue.projectionKey ?? ""}`}>
          <strong className={severityClass(issue.severity)}>{issue.severity}</strong>
          <span>
            {issue.code}: {issue.summary}
          </span>
        </li>
      ))}
    </ul>
  );
}

function severityClass(severity: ProjectionIssue["severity"]): string {
  if (severity === "error") return styles.issueError;
  if (severity === "warning") return styles.issueWarning;
  return styles.issueInfo;
}
