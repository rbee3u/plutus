import { useSessionActions } from "@/shared/lib/session";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { type FormEvent, useId, useRef, useState } from "react";
import { AlertCircle, Check } from "lucide-react";
import { problemError, problemMessage, putAuthPassword } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { PasswordInput } from "@/shared/ui/PasswordInput";
import styles from "../styles.module.css";
import shared from "@/shared/styles/page.module.css";
import passwordStyles from "./PasswordForm.module.css";

export function PasswordForm({ onError: setError }: { onError: (error: string | null) => void }) {
  const write = useAuthenticatedWrite();
  const { endSession } = useSessionActions();
  const currentPasswordRef = useRef<HTMLInputElement>(null);
  const currentPasswordId = useId();
  const newPasswordId = useId();
  const confirmPasswordId = useId();
  const passwordRequirementsId = useId();

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");

  const [currentPasswordError, setCurrentPasswordError] = useState<string | null>(null);

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

  return (
    <form
      className={`${shared.form} ${styles.settingsCard}`}
      onSubmit={(event) => void onChangePassword(event)}
    >
      <h2>Change password</h2>

      <div className={shared.field}>
        <label htmlFor={currentPasswordId}>
          <span>Current password</span>
        </label>
        <PasswordInput
          id={currentPasswordId}
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
      </div>

      <div className={shared.field}>
        <label htmlFor={newPasswordId}>
          <span>New password</span>
        </label>
        <PasswordInput
          id={newPasswordId}
          value={newPassword}
          autoComplete="new-password"
          minLength={8}
          maxLength={128}
          onChange={(event) => setNewPassword(event.target.value)}
          aria-invalid={isSameAsCurrent}
          aria-describedby={`${passwordRequirementsId}${isSameAsCurrent ? " new-password-error" : ""}`}
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
        <div className={passwordStyles.rulesRow} id={passwordRequirementsId}>
          <span
            className={`${passwordStyles.ruleBadge} ${isLengthValid ? passwordStyles.ruleBadgeMet : ""}`}
          >
            {isLengthValid ? <Check size={iconSize.xs} aria-hidden="true" /> : null}
            8+ chars
          </span>
          <span
            className={`${passwordStyles.ruleBadge} ${hasUppercase ? passwordStyles.ruleBadgeMet : ""}`}
          >
            {hasUppercase ? <Check size={iconSize.xs} aria-hidden="true" /> : null}
            A–Z
          </span>
          <span
            className={`${passwordStyles.ruleBadge} ${hasLowercase ? passwordStyles.ruleBadgeMet : ""}`}
          >
            {hasLowercase ? <Check size={iconSize.xs} aria-hidden="true" /> : null}
            a–z
          </span>
          <span
            className={`${passwordStyles.ruleBadge} ${hasNumberOrSymbol ? passwordStyles.ruleBadgeMet : ""}`}
          >
            {hasNumberOrSymbol ? <Check size={iconSize.xs} aria-hidden="true" /> : null}
            0–9 / symbol
          </span>
        </div>
      </div>

      <div className={shared.field}>
        <label htmlFor={confirmPasswordId}>
          <span>Confirm new password</span>
        </label>
        <PasswordInput
          id={confirmPasswordId}
          value={confirmPassword}
          autoComplete="new-password"
          onChange={(event) => setConfirmPassword(event.target.value)}
          aria-invalid={isConfirmMismatch}
          aria-describedby={isConfirmMismatch ? "confirm-password-error" : undefined}
          required
        />
        {isConfirmMismatch ? (
          <span
            id="confirm-password-error"
            className={`${passwordStyles.fieldHint} ${passwordStyles.fieldHintError}`}
            role="alert"
          >
            <AlertCircle size={iconSize.xs} />
            Passwords do not match
          </span>
        ) : null}
      </div>

      <div className={shared.actions}>
        <ActionButton type="submit" tone="primary" disabled={!canSubmit}>
          Change password
        </ActionButton>
      </div>
    </form>
  );
}
