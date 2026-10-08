import { useState } from "react";
import { PasswordForm } from "../components/PasswordForm";
import { SessionsPanel } from "../components/SessionsPanel";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";

export function SecuritySettingsPage() {
  const [error, setError] = useState<string | null>(null);
  return (
    <section className={styles.page}>
      <div className={shared.panelHeader} style={{ flexWrap: "wrap", gap: "var(--space-md)" }}>
        <div>
          <h2 style={{ margin: 0 }}>Account & Security</h2>
          <p className={shared.muted} style={{ margin: "4px 0 0", fontSize: "var(--text-sm)" }}>
            Manage authentication credentials and review active browser sessions.
          </p>
        </div>
      </div>
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
