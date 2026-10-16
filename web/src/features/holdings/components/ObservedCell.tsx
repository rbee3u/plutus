import type { Issue } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import shared from "@/shared/styles/page.module.css";

export function IssuesCell({
  observedAt,
  freshness,
  issues = [],
  name,
  timezone,
}: {
  observedAt?: string | null;
  freshness: string | null;
  issues?: Issue[];
  name: string;
  timezone?: string;
}) {
  const isStale = freshness === "stale";
  const stale = issues.find((issue) => issue.code === "stale");
  const anomalies = issues.filter((issue) => issue.code !== "stale");

  if (!isStale && anomalies.length === 0) {
    return <span className={shared.muted}>—</span>;
  }

  return (
    <span className={shared.statusGroup}>
      {isStale ? (
        <HoverDetails label={<Badge tone="warn">Stale</Badge>} name={`Stale report for ${name}`}>
          <p>{stale?.detail ?? "An earlier successful account report is carried forward."}</p>
          {observedAt ? (
            <p className={shared.cellMeta}>Observed {formatInstant(observedAt, timezone)}</p>
          ) : null}
        </HoverDetails>
      ) : null}
      {anomalies.length ? (
        <HoverDetails
          label={
            <Badge tone="warn">
              {formatCount(anomalies.length)} {anomalies.length === 1 ? "issue" : "issues"}
            </Badge>
          }
          name={`Data issues for ${name}`}
        >
          {anomalies.map((issue, index) => (
            <p key={`${issue.code}-${index}`}>{issue.detail}</p>
          ))}
        </HoverDetails>
      ) : null}
    </span>
  );
}

export const ObservedCell = IssuesCell;
