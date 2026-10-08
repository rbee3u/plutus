import { deleteAuthSession, problemMessage } from "@/api";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { formatInstant } from "@/shared/lib/datetime";
import { clientSummary } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { useSessions } from "../hooks/useSessions";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function SessionsPanel({ onError: setError }: { onError: (error: string | null) => void }) {
  const write = useAuthenticatedWrite();
  const sessions = useSessions(setError);
  async function revoke(id: string) {
    setError(null);
    try {
      const result = await write(() => deleteAuthSession({ path: { sessionId: id } }));
      if (result.error) {
        setError(problemMessage(result.error, "The session could not be revoked."));
        return;
      }
      await sessions.refetch();
    } catch (cause) {
      setError(
        problemMessage(cause, "The session could not be revoked. Check the network and retry."),
      );
    }
  }

  return (
    <section className={`${shared.panel} ${styles.settingsCard}`}>
      <h2>Sessions</h2>
      <ul className={styles.sessions}>
        {(sessions.data?.items ?? []).map((session) => (
          <li key={session.id}>
            <div>
              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: "var(--space-xs)",
                  flexWrap: "wrap",
                }}
              >
                <strong>
                  {session.clientLabel ? clientSummary(session.clientLabel) : "Unknown client"}
                </strong>
                {session.current ? (
                  <span className={styles.currentSessionBadge}>Current</span>
                ) : null}
              </div>
              <p className={shared.muted} title={session.clientLabel ?? undefined}>
                Signed in {formatInstant(session.createdAt)} · last seen{" "}
                {formatInstant(session.lastSeenAt)}
              </p>
            </div>
            {!session.current ? (
              <ActionButton tone="dangerQuiet" onClick={() => void revoke(session.id)}>
                Revoke
              </ActionButton>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}
