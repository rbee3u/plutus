import { useState } from "react";
import { PasswordForm } from "../components/PasswordForm";
import { SessionsPanel } from "../components/SessionsPanel";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function SecuritySettingsPage() {
  const [error, setError] = useState<string | null>(null);
  return (
    <section className={styles.page}>
      {error ? (
        <p className={shared.alert} role="alert">
          {error}
        </p>
      ) : null}
      <PasswordForm onError={setError} />
      <SessionsPanel onError={setError} />
    </section>
  );
}
