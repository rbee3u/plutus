import type { Issue } from "@/api";
import { formatCount } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { Badge } from "@/shared/ui/Badge";
import { HoverDetails } from "@/shared/ui/HoverDetails";
import shared from "@/shared/styles/page.module.css";

export function ObservedCell({
  observedAt,
  freshness,
  issues = [],
  name,
  timezone,
}: {
  observedAt?: string | null;
  freshness: string;
  issues?: Issue[];
  name: string;
  timezone?: string;
}) {
  const stale = issues.find((issue) => issue.code === "stale");
  const anomalies = issues.filter((issue) => issue.code !== "stale");
  return (
    <span className={shared.statusGroup}>
      <span
        className={shared.muted}
        title={`Account Report observed at ${observedAt ?? "an unknown time"}; times in ${timezone ?? "the Portfolio timezone"}.`}
      >
        {formatInstant(observedAt, timezone)}
      </span>
      {freshness === "stale" ? (
        <HoverDetails label={<Badge tone="warn">Stale</Badge>} name={`Stale report for ${name}`}>
          <p>{stale?.detail ?? "An earlier successful Account Report is carried forward."}</p>
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
