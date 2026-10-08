import type { UseQueryResult } from "@tanstack/react-query";
import { problemError } from "@/api";
import { ActionButton } from "@/shared/ui/ActionButton";
import styles from "@/shared/styles/page.module.css";

export function QueryRecovery({
  query,
  fallback,
}: {
  query: Pick<UseQueryResult<unknown, unknown>, "error" | "data" | "isFetching" | "refetch">;
  fallback: string;
}) {
  return (
    <RequestRecovery
      error={query.error}
      hasData={Boolean(query.data)}
      busy={query.isFetching}
      fallback={fallback}
      onRetry={() => {
        void query.refetch();
      }}
    />
  );
}

/** Explicit recovery for combined requests or locally managed state. */
export function RequestRecovery({
  error,
  hasData,
  onRetry,
  busy,
  fallback,
}: {
  error: unknown;
  hasData: boolean;
  onRetry: () => void;
  busy?: boolean;
  fallback: string;
}) {
  if (!error) {
    return null;
  }
  const problem = problemError(error, fallback);
  return (
    <div className={hasData ? styles.notice : styles.alert} role="alert">
      {hasData ? <p>Showing the last successful result. It may be stale.</p> : null}
      <p>{problem.message}</p>
      {problem.traceId ? <p className={styles.muted}>Trace ID {problem.traceId}</p> : null}
      <div className={styles.actions}>
        <ActionButton type="button" disabled={busy} onClick={onRetry}>
          Retry
        </ActionButton>
      </div>
    </div>
  );
}
