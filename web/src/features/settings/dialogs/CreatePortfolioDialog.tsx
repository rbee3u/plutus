import { createPortfolio, keys, problemMessage, type Portfolio } from "@/api";
import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { useSessionScope } from "@/shared/lib/session";
import { useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useRef, useState } from "react";
import { newIdempotencyKey } from "@/shared/lib/id";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import pageStyles from "@/shared/styles/page.module.css";

export function CreatePortfolioDialog({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (portfolio: Portfolio) => void;
}) {
  const write = useAuthenticatedWrite();
  const queryClient = useQueryClient();
  const { userId } = useSessionScope();
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const createKey = useRef(newIdempotencyKey());
  async function onCreate(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const result = await write(() =>
        createPortfolio({
          headers: { "Idempotency-Key": createKey.current },
          body: { name },
        }),
      );
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The portfolio could not be created."));
        return;
      }
      createKey.current = newIdempotencyKey();
      await queryClient.invalidateQueries({ queryKey: keys.portfolios(userId) });
      setName("");
      onClose();
      onCreated(result.data);
    } catch (cause) {
      setError(
        problemMessage(cause, "The portfolio could not be created. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open={open}
      title="New portfolio"
      onClose={() => {
        if (!busy) {
          onClose();
          setError(null);
        }
      }}
    >
      <form className={pageStyles.form} onSubmit={(event) => void onCreate(event)}>
        {error ? (
          <p className={pageStyles.alert} role="alert">
            {error}
          </p>
        ) : null}
        <label className={pageStyles.field}>
          <span>Name</span>
          <TextInput
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={120}
            required
          />
        </label>
        <div className={pageStyles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            Create
          </ActionButton>
          <ActionButton
            type="button"
            tone="ghost"
            disabled={busy}
            onClick={() => {
              onClose();
              setError(null);
            }}
          >
            Cancel
          </ActionButton>
        </div>
      </form>
    </Dialog>
  );
}
