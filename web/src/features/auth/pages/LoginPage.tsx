import { useQueryClient } from "@tanstack/react-query";
import { clearSession, useCurrentUser } from "@/shared/lib/session";
import { type FormEvent, useEffect, useId, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { keys, postAuthLogin, problemMessage, executeWrite } from "@/api";
import { LAST_PORTFOLIO_KEY } from "@/shared/lib/preferences";
import { readPreference } from "@/shared/lib/preferences";
import { iconSize } from "@/shared/icons/iconSizes";
import { PlutusIcon } from "@/shared/icons/PlutusIcon";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import styles from "@/features/auth/styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function LoginPage() {
  const passwordId = useId();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const location = useLocation();
  const stateNotice = (location.state as { notice?: string } | null)?.notice ?? null;
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [notice, setNotice] = useState<string | null>(stateNotice);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    document.title = "Sign in · Plutus";
  }, []);
  useEffect(() => {
    if (user.data) void navigate("/", { replace: true });
  }, [navigate, user.data]);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const result = await executeWrite(() => postAuthLogin({ body: { username, password } }));
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "Sign-in failed."));
        return;
      }
      await clearSession(queryClient);
      queryClient.setQueryData(keys.authMe(), result.data.user);
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
        <div className={styles.cardBrand}>
          <PlutusIcon size={iconSize.lg} />
          <span>Plutus</span>
        </div>
        <h1>Sign in</h1>
        <p className={styles.cardSubtitle}>Track your portfolio value over time.</p>
        {notice ? (
          <p className={shared.notice} role="status">
            {notice}
          </p>
        ) : null}
        {error ? (
          <p className={shared.alert} role="alert">
            {error}
          </p>
        ) : null}
        <label className={shared.field}>
          <span>Username</span>
          <TextInput
            value={username}
            placeholder="Enter username"
            autoComplete="username"
            onChange={(event) => setUsername(event.target.value)}
            required
          />
        </label>
        <div className={shared.field}>
          <label htmlFor={passwordId}>Password</label>
          <PasswordInput
            id={passwordId}
            value={password}
            placeholder="Enter password"
            autoComplete="current-password"
            onChange={(event) => setPassword(event.target.value)}
            required
          />
        </div>
        <div className={styles.authActions}>
          <ActionButton type="submit" tone="primary" disabled={busy} className={styles.authSubmit}>
            {busy ? "Signing in..." : "Sign in"}
          </ActionButton>
        </div>
      </form>
    </div>
  );
}
