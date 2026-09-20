import { type FormEvent, useEffect, useState } from "react";
import { Link } from "react-router";
import { getAuthRegistrationStatus, postAuthRegister } from "@/api/generated/sdk.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { ActionButton } from "@/shared/ui/ActionButton";
import { TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

export function RegisterPage() {
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [created, setCreated] = useState(false);

  useEffect(() => {
    document.title = "Register - Plutus";
    void (async () => {
      try {
        await refreshCsrf();
        const result = await getAuthRegistrationStatus();
        setEnabled(result.data?.enabled ?? false);
      } catch (cause) {
        setError(problemMessage(cause, "Registration status could not be loaded."));
        setEnabled(false);
      }
    })();
  }, []);

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await postAuthRegister({ body: { username, password } });
      if (result.error) {
        setError(problemMessage(result.error, "Registration failed."));
        return;
      }
      setCreated(true);
    } catch (cause) {
      setError(problemMessage(cause, "Registration failed. Check the network and retry."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.auth}>
      <div className={styles.card}>
        <h1>Create an account</h1>
        {enabled === null ? (
          <p className={styles.lead}>Checking whether public registration is open.</p>
        ) : enabled === false ? (
          <>
            <p className={styles.notice}>
              Public registration is closed. Ask an operator to create the first user with{" "}
              <code>plutus user create</code>.
            </p>
            <Link to="/login">Back to sign in</Link>
          </>
        ) : (
          <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
            <p className={styles.lead}>
              Choose a username and a password of at least 12 characters.
            </p>
            {error ? (
              <p className={styles.alert} role="alert">
                {error}
              </p>
            ) : null}
            {created ? (
              <p className={styles.notice}>
                The account was created. You can now <Link to="/login">sign in</Link>.
              </p>
            ) : (
              <>
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
                    autoComplete="new-password"
                    onChange={(event) => setPassword(event.target.value)}
                    required
                    minLength={12}
                  />
                </label>
                <div className={styles.actions}>
                  <ActionButton type="submit" tone="primary" disabled={busy || enabled !== true}>
                    Register
                  </ActionButton>
                  <Link to="/login">Sign in</Link>
                </div>
              </>
            )}
          </form>
        )}
      </div>
    </div>
  );
}
