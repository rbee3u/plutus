import { type FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router";
import { AlertCircle, Check, Circle } from "lucide-react";
import { deleteAuthSession, getAuthSessions, putAuthPassword } from "@/api/generated/sdk.gen";
import { problemError, problemMessage, refreshCsrf } from "@/api/http";
import { iconSize } from "@/shared/icons/iconSizes";
import { formatInstant } from "@/shared/lib/datetime";
import { clientSummary } from "@/shared/lib/text";
import { ActionButton } from "@/shared/ui/ActionButton";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import styles from "@/features/pages.module.css";
import secStyles from "@/features/settings/SecuritySettingsPage.module.css";

interface SessionRow {
  id: string;
  current: boolean;
  createdAt: string;
  lastSeenAt: string;
  clientLabel?: string | null;
}

function computeStrength(password: string): { score: number; label: string } {
  if (!password) {
    return { score: 0, label: "" };
  }
  let points = 0;
  if (password.length >= 8) points++;
  if (/[a-z]/.test(password) && /[A-Z]/.test(password)) points++;
  if (/[0-9]/.test(password)) points++;
  if (/[^a-zA-Z0-9]/.test(password) || password.length >= 14) points++;

  const labels = ["", "Weak", "Fair", "Good", "Strong"];
  return { score: points, label: labels[points] ?? "Strong" };
}

export function SecuritySettingsPage() {
  const navigate = useNavigate();
  const currentPasswordRef = useRef<HTMLInputElement>(null);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [currentPasswordError, setCurrentPasswordError] = useState<string | null>(null);
  const [newPasswordFocused, setNewPasswordFocused] = useState(false);

  const [sessions, setSessions] = useState<SessionRow[]>([]);
  const [error, setError] = useState<string | null>(null);
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

  const isLengthValid = newPassword.length >= 8 && newPassword.length <= 128;
  const hasUppercase = /[A-Z]/.test(newPassword);
  const hasLowercase = /[a-z]/.test(newPassword);
  const hasNumberOrSymbol = /[0-9]|[^a-zA-Z0-9]/.test(newPassword);
  const isComplexityValid = isLengthValid && hasUppercase && hasLowercase && hasNumberOrSymbol;

  const isDifferentFromCurrent =
    currentPassword.length > 0 && newPassword.length > 0 ? newPassword !== currentPassword : true;
  const isSameAsCurrent =
    currentPassword.length > 0 && newPassword.length > 0 && newPassword === currentPassword;

  const isConfirmTouched = confirmPassword.length > 0;
  const isConfirmMatching = isConfirmTouched && confirmPassword === newPassword;
  const isConfirmMismatch = isConfirmTouched && confirmPassword !== newPassword;

  const { score: strengthScore, label: strengthLabel } = computeStrength(newPassword);

  const canSubmit =
    currentPassword.length > 0 &&
    isComplexityValid &&
    isDifferentFromCurrent &&
    isConfirmMatching &&
    !busy;

  async function onChangePassword(event: FormEvent) {
    event.preventDefault();
    if (!canSubmit) {
      return;
    }
    setBusy(true);
    setError(null);
    setCurrentPasswordError(null);

    try {
      await refreshCsrf();
      const result = await putAuthPassword({ body: { currentPassword, newPassword } });

      if (result.response?.status === 401) {
        const err = problemError(result.error, "");
        if (err.code === "invalid_credentials") {
          setCurrentPasswordError("Current password is incorrect.");
          currentPasswordRef.current?.focus();
          return;
        }
        void navigate("/login", { replace: true });
        return;
      }

      if (result.error) {
        setError(problemMessage(result.error, "The password could not be changed."));
        return;
      }

      // Password changed successfully: all sessions revoked, redirect to login
      void navigate("/login", {
        replace: true,
        state: {
          notice:
            "Password changed successfully. All active sessions have been signed out. Please sign in with your new password.",
        },
      });
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

  const scoreClass =
    strengthScore === 1
      ? secStyles.scoreWeak
      : strengthScore === 2
        ? secStyles.scoreFair
        : strengthScore === 3
          ? secStyles.scoreGood
          : strengthScore === 4
            ? secStyles.scoreStrong
            : "";

  const barColorClass =
    strengthScore === 1
      ? secStyles.strengthBarWeak
      : strengthScore === 2
        ? secStyles.strengthBarFair
        : strengthScore === 3
          ? secStyles.strengthBarGood
          : strengthScore === 4
            ? secStyles.strengthBarStrong
            : "";

  return (
    <section className={styles.page}>
      <p className={styles.lead}>
        Changing your password will sign you out across all devices and require you to sign in
        again.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      <form className={styles.form} onSubmit={(event) => void onChangePassword(event)}>
        <h2>Change password</h2>

        <label className={styles.field}>
          <span>Current password</span>
          <PasswordInput
            ref={currentPasswordRef}
            value={currentPassword}
            autoComplete="current-password"
            onChange={(event) => {
              setCurrentPassword(event.target.value);
              if (currentPasswordError) {
                setCurrentPasswordError(null);
              }
            }}
            aria-invalid={Boolean(currentPasswordError)}
            aria-describedby={currentPasswordError ? "current-password-error" : undefined}
            required
          />
          {currentPasswordError ? (
            <span
              id="current-password-error"
              className={`${secStyles.fieldHint} ${secStyles.fieldHintError}`}
              role="alert"
            >
              <AlertCircle size={iconSize.xs} />
              {currentPasswordError}
            </span>
          ) : null}
        </label>

        <label className={styles.field}>
          <span>New password</span>
          <PasswordInput
            value={newPassword}
            autoComplete="new-password"
            minLength={8}
            maxLength={128}
            onFocus={() => setNewPasswordFocused(true)}
            onChange={(event) => setNewPassword(event.target.value)}
            aria-invalid={isSameAsCurrent}
            aria-describedby={isSameAsCurrent ? "new-password-error" : undefined}
            required
          />
          {isSameAsCurrent ? (
            <span
              id="new-password-error"
              className={`${secStyles.fieldHint} ${secStyles.fieldHintError}`}
              role="alert"
            >
              <AlertCircle size={iconSize.xs} />
              New password must differ from current password
            </span>
          ) : null}
          {newPasswordFocused || newPassword.length > 0 ? (
            <div className={secStyles.strengthMeter}>
              {strengthLabel ? (
                <div className={secStyles.strengthHeader}>
                  <span>Password strength</span>
                  <span className={`${secStyles.strengthScore} ${scoreClass}`}>
                    {strengthLabel}
                  </span>
                </div>
              ) : null}
              <div className={secStyles.strengthBars}>
                <div
                  className={`${secStyles.strengthBar} ${strengthScore >= 1 ? barColorClass : ""}`}
                />
                <div
                  className={`${secStyles.strengthBar} ${strengthScore >= 2 ? barColorClass : ""}`}
                />
                <div
                  className={`${secStyles.strengthBar} ${strengthScore >= 3 ? barColorClass : ""}`}
                />
                <div
                  className={`${secStyles.strengthBar} ${strengthScore >= 4 ? barColorClass : ""}`}
                />
              </div>
              <ul className={secStyles.rulesList}>
                <li
                  className={`${secStyles.ruleItem} ${isLengthValid ? secStyles.rulePassed : ""}`}
                >
                  {isLengthValid ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                  <span>8 to 128 characters</span>
                </li>
                <li className={`${secStyles.ruleItem} ${hasUppercase ? secStyles.rulePassed : ""}`}>
                  {hasUppercase ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                  <span>An uppercase letter (A–Z)</span>
                </li>
                <li className={`${secStyles.ruleItem} ${hasLowercase ? secStyles.rulePassed : ""}`}>
                  {hasLowercase ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                  <span>A lowercase letter (a–z)</span>
                </li>
                <li
                  className={`${secStyles.ruleItem} ${hasNumberOrSymbol ? secStyles.rulePassed : ""}`}
                >
                  {hasNumberOrSymbol ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                  <span>A number or symbol</span>
                </li>
              </ul>
            </div>
          ) : null}
        </label>

        <label className={styles.field}>
          <span>Confirm new password</span>
          <PasswordInput
            value={confirmPassword}
            autoComplete="new-password"
            onChange={(event) => setConfirmPassword(event.target.value)}
            aria-invalid={isConfirmMismatch}
            aria-describedby={
              isConfirmMismatch
                ? "confirm-password-error"
                : isConfirmMatching
                  ? "confirm-password-match"
                  : undefined
            }
            required
          />
          {isConfirmMatching ? (
            <span
              id="confirm-password-match"
              className={`${secStyles.fieldHint} ${secStyles.fieldHintSuccess}`}
            >
              <Check size={iconSize.xs} />
              Passwords match
            </span>
          ) : isConfirmMismatch ? (
            <span
              id="confirm-password-error"
              className={`${secStyles.fieldHint} ${secStyles.fieldHintError}`}
              role="alert"
            >
              <AlertCircle size={iconSize.xs} />
              Passwords do not match
            </span>
          ) : null}
        </label>

        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={!canSubmit}>
            Change password
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
