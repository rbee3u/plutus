import { useSessionActions } from "@/shared/lib/session";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useRef, useState } from "react";
import { AlertCircle, Check, Circle } from "lucide-react";
import { problemError, problemMessage, putAuthPassword } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";
import passwordStyles from "./PasswordForm.module.css";

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

export function PasswordForm({ onError: setError }: { onError: (error: string | null) => void }) {
  const write = useAuthenticatedWrite();
  const { endSession } = useSessionActions();
  const currentPasswordRef = useRef<HTMLInputElement>(null);

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [currentPasswordError, setCurrentPasswordError] = useState<string | null>(null);
  const [newPasswordFocused, setNewPasswordFocused] = useState(false);

  const [busy, setBusy] = useState(false);
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
      const result = await write(
        () => putAuthPassword({ body: { currentPassword, newPassword } }),
        { acceptUnauthorized: (error) => problemError(error, "").code === "invalid_credentials" },
      );

      if (result.response?.status === 401) {
        const err = problemError(result.error, "");
        if (err.code === "invalid_credentials") {
          setCurrentPasswordError("Current password is incorrect.");
          currentPasswordRef.current?.focus();
          return;
        }
      }

      if (result.error) {
        setError(problemMessage(result.error, "The password could not be changed."));
        return;
      }

      // Password changed successfully: all sessions revoked, redirect to login
      await endSession({
        notice:
          "Password changed successfully. All active sessions have been signed out. Please sign in with your new password.",
      });
    } catch (cause) {
      setError(
        problemMessage(cause, "The password could not be changed. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  const scoreClass =
    strengthScore === 1
      ? passwordStyles.scoreWeak
      : strengthScore === 2
        ? passwordStyles.scoreFair
        : strengthScore === 3
          ? passwordStyles.scoreGood
          : strengthScore === 4
            ? passwordStyles.scoreStrong
            : "";

  const barColorClass =
    strengthScore === 1
      ? passwordStyles.strengthBarWeak
      : strengthScore === 2
        ? passwordStyles.strengthBarFair
        : strengthScore === 3
          ? passwordStyles.strengthBarGood
          : strengthScore === 4
            ? passwordStyles.strengthBarStrong
            : "";

  return (
    <form
      className={`${shared.form} ${styles.settingsCard}`}
      onSubmit={(event) => void onChangePassword(event)}
    >
      <h2>Change password</h2>

      <label className={shared.field}>
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
            className={`${passwordStyles.fieldHint} ${passwordStyles.fieldHintError}`}
            role="alert"
          >
            <AlertCircle size={iconSize.xs} />
            {currentPasswordError}
          </span>
        ) : null}
      </label>

      <label className={shared.field}>
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
            className={`${passwordStyles.fieldHint} ${passwordStyles.fieldHintError}`}
            role="alert"
          >
            <AlertCircle size={iconSize.xs} />
            New password must differ from current password
          </span>
        ) : null}
        {newPasswordFocused || newPassword.length > 0 ? (
          <div className={passwordStyles.strengthMeter}>
            {strengthLabel ? (
              <div className={passwordStyles.strengthHeader}>
                <span>Password strength</span>
                <span className={`${passwordStyles.strengthScore} ${scoreClass}`}>
                  {strengthLabel}
                </span>
              </div>
            ) : null}
            <div className={passwordStyles.strengthBars}>
              <div
                className={`${passwordStyles.strengthBar} ${strengthScore >= 1 ? barColorClass : ""}`}
              />
              <div
                className={`${passwordStyles.strengthBar} ${strengthScore >= 2 ? barColorClass : ""}`}
              />
              <div
                className={`${passwordStyles.strengthBar} ${strengthScore >= 3 ? barColorClass : ""}`}
              />
              <div
                className={`${passwordStyles.strengthBar} ${strengthScore >= 4 ? barColorClass : ""}`}
              />
            </div>
            <ul className={passwordStyles.rulesList}>
              <li
                className={`${passwordStyles.ruleItem} ${isLengthValid ? passwordStyles.rulePassed : ""}`}
              >
                {isLengthValid ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                <span>8 to 128 characters</span>
              </li>
              <li
                className={`${passwordStyles.ruleItem} ${hasUppercase ? passwordStyles.rulePassed : ""}`}
              >
                {hasUppercase ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                <span>An uppercase letter (A–Z)</span>
              </li>
              <li
                className={`${passwordStyles.ruleItem} ${hasLowercase ? passwordStyles.rulePassed : ""}`}
              >
                {hasLowercase ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                <span>A lowercase letter (a–z)</span>
              </li>
              <li
                className={`${passwordStyles.ruleItem} ${hasNumberOrSymbol ? passwordStyles.rulePassed : ""}`}
              >
                {hasNumberOrSymbol ? <Check size={iconSize.xs} /> : <Circle size={iconSize.xs} />}
                <span>A number or symbol</span>
              </li>
            </ul>
          </div>
        ) : null}
      </label>

      <label className={shared.field}>
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
            className={`${passwordStyles.fieldHint} ${passwordStyles.fieldHintSuccess}`}
          >
            <Check size={iconSize.xs} />
            Passwords match
          </span>
        ) : isConfirmMismatch ? (
          <span
            id="confirm-password-error"
            className={`${passwordStyles.fieldHint} ${passwordStyles.fieldHintError}`}
            role="alert"
          >
            <AlertCircle size={iconSize.xs} />
            Passwords do not match
          </span>
        ) : null}
      </label>

      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={!canSubmit}>
          Change password
        </ActionButton>
      </div>
    </form>
  );
}
