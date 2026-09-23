import { type FormEvent, useCallback, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { deleteAuthSession, getAuthSessions, putAuthPassword } from "@/api/generated/sdk.gen";
import { problemError, problemMessage, refreshCsrf } from "@/api/http";
import { formatInstant } from "@/shared/lib/datetime";
import { clientSummary } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface SessionRow {
  id: string;
  current: boolean;
  createdAt: string;
  lastSeenAt: string;
  clientLabel?: string | null;
}

export function SecuritySettingsPage() {
  const navigate = useNavigate();
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [sessions, setSessions] = useState<SessionRow[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const loadSessions = useCallback(async () => {
    try {
      const result = await getAuthSessions();
      if (result.response?.status === 401) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "Sessions could not be loaded."));
        return;
      }
      setSessions(result.data.items);
    } catch (cause) {
      setError(problemMessage(cause, "Sessions could not be loaded. Check the network and retry."));
    }
  }, [navigate]);

  useEffect(() => {
    void (async () => {
      await loadSessions();
    })();
  }, [loadSessions]);

  async function onChangePassword(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const result = await putAuthPassword({ body: { currentPassword, newPassword } });
      // A wrong current password is also a 401; only a lost session goes back to the login page.
      if (
        result.response?.status === 401 &&
        problemError(result.error, "").code !== "invalid_credentials"
      ) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The password could not be changed."));
        return;
      }
      setCurrentPassword("");
      setNewPassword("");
      setNotice("Password updated. Other sessions were revoked.");
      await loadSessions();
    } catch (cause) {
      setError(
        problemMessage(cause, "The password could not be changed. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  async function revoke(id: string) {
    setError(null);
    try {
      await refreshCsrf();
      const result = await deleteAuthSession({ path: { sessionId: id } });
      if (result.response?.status === 401) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The session could not be revoked."));
        return;
      }
      await loadSessions();
    } catch (cause) {
      setError(
        problemMessage(cause, "The session could not be revoked. Check the network and retry."),
      );
    }
  }

  return (
    <section className={styles.page}>
      <p className={styles.lead}>
        Changing the password signs every other session out. Sessions are the browsers signed in as
        this account.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? (
        <p className={styles.notice} role="status">
          {notice}
        </p>
      ) : null}
      <form className={styles.form} onSubmit={(event) => void onChangePassword(event)}>
        <h2>Change password</h2>
        <label className={styles.field}>
          <span>Current password</span>
          <TextInput
            type="password"
            value={currentPassword}
            autoComplete="current-password"
            onChange={(event) => setCurrentPassword(event.target.value)}
            required
          />
        </label>
        <label className={styles.field}>
          <span>New password</span>
          <TextInput
            type="password"
            value={newPassword}
            autoComplete="new-password"
            minLength={8}
            onChange={(event) => setNewPassword(event.target.value)}
            required
          />
        </label>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Update password
          </ActionButton>
        </div>
      </form>
      <section className={styles.panel}>
        <h2>Sessions</h2>
        <ul className={styles.sessions}>
          {sessions.map((session) => (
            <li key={session.id}>
              <div>
                <strong>
                  {session.clientLabel ? clientSummary(session.clientLabel) : "Unknown client"}
                  {session.current ? " · this session" : ""}
                </strong>
                <p className={styles.muted} title={session.clientLabel ?? undefined}>
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
    </section>
  );
}
