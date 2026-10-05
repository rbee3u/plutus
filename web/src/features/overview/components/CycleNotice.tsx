import { NumericText } from "@/shared/ui/NumericText";
import type { CollectionCycle } from "@/api";
import { formatUsd } from "@/shared/lib/amount";
import { formatInstant } from "@/shared/lib/datetime";
import { shortId } from "@/shared/lib/id";
import { StatusBadge } from "@/shared/ui/Badge";
import { isTerminalStatus } from "../useCollectNow";
import shared from "@/shared/styles/page.module.css";

/** The banner of the Collection Cycle started from this page. */
export function CycleNotice({ cycle, timezone }: { cycle: CollectionCycle; timezone?: string }) {
  const finished = isTerminalStatus(cycle.status);
  return (
    <p className={cycle.status === "failed" ? shared.alert : shared.notice} role="status">
      Collection Cycle {shortId(cycle.id)} <StatusBadge value={cycle.status} />{" "}
      {finished
        ? `at ${formatInstant(cycle.finishedAt, timezone)}.`
        : "· this page refreshes when it finishes."}
      <NumericText>
        {cycle.point
          ? ` Point total ${formatUsd(cycle.point.totalUsd)} (${cycle.point.completeness}).`
          : ""}
      </NumericText>
      {cycle.lastErrorClass
        ? ` ${cycle.lastErrorClass}${cycle.lastDetail ? `: ${cycle.lastDetail}` : ""}`
        : ""}
    </p>
  );
}
