import { type FormEvent, useEffect, useState } from "react";
import { useNavigate } from "react-router";
import { getAuthMe, postAuthLogin } from "@/api/generated/sdk.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { readPreference } from "@/shared/lib/preferences";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

export function LoginPage() {
  const navigate = useNavigate();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    document.title = "Sign in - Plutus";
    void (async () => {
      try {
        await refreshCsrf();
        const me = await getAuthMe();
        if (me.data) {
          void navigate("/", { replace: true });
        }
      } catch {
        // Stay on the login screen when the session is missing.
      }
    })();
  }, [navigate]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await postAuthLogin({ body: { username, password } });
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "Sign-in failed."));
        return;
      }
      const remembered = readPreference(LAST_PORTFOLIO_KEY);
      void navigate(remembered ? `/p/${remembered}/overview` : "/", { replace: true });
    } catch (cause) {
      setError(problemMessage(cause, "Sign-in failed. Check the network and retry."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.auth}>
      <form className={styles.card} onSubmit={(event) => void onSubmit(event)}>
        <h1>Sign in to Plutus</h1>
        <p className={styles.lead}>
          Create users with the CLI command <code>plutus user create</code>.
        </p>
        {error ? (
          <p className={styles.alert} role="alert">
            {error}
          </p>
        ) : null}
        <label className={styles.field}>
          <span>Username</span>
          <TextInput
            value={username}
            autoComplete="username"
            onChange={(event) => setUsername(event.target.value)}
            required
          />
        </label>
        <label className={styles.field}>
          <span>Password</span>
          <TextInput
            type="password"
            value={password}
            autoComplete="current-password"
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </label>
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Sign in
          </ActionButton>
        </div>
      </form>
    </div>
  );
}
