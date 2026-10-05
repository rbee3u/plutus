import type { Identity, Platform } from "@/api";
import { CreateForm } from "./CreateForm";
import { CredentialsForm } from "./CredentialsForm";
import { EditForm } from "./EditForm";
import { Dialog } from "@/shared/ui/Dialog";

export type IdentityDialogInput =
  { kind: "create" } | { kind: "edit"; item: Identity } | { kind: "credentials"; item: Identity };

export type IdentityDialogRequest = IdentityDialogInput & { nonce: number };

interface DialogProps {
  portfolioId: string;
  request: IdentityDialogRequest | null;
  platforms: Platform[];
  onClose: () => void;
  onSaved: (message: string) => Promise<void>;
}

export function IdentityDialog({ portfolioId, request, platforms, onClose, onSaved }: DialogProps) {
  const title =
    request?.kind === "edit"
      ? `Edit ${request.item.displayName}`
      : request?.kind === "credentials"
        ? `Credentials for ${request.item.displayName}`
        : "New identity";
  return (
    <Dialog open={request !== null} title={title} onClose={onClose}>
      {request?.kind === "create" ? (
        <CreateForm
          key={request.nonce}
          portfolioId={portfolioId}
          platforms={platforms}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "edit" ? (
        <EditForm
          key={request.nonce}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "credentials" ? (
        <CredentialsForm
          key={request.nonce}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}
