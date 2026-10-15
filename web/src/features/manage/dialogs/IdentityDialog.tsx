import type { Identity, Platform } from "@/api";
import { CreateIdentityForm } from "./CreateIdentityForm";
import { IdentityCredentialsForm } from "./IdentityCredentialsForm";
import { EditIdentityForm } from "./EditIdentityForm";
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
  const credentialPlatform =
    request?.kind === "credentials"
      ? platforms.find((platform) => platform.id === request.item.platform)
      : undefined;
  return (
    <Dialog open={request !== null} title={title} onClose={onClose}>
      {request?.kind === "create" ? (
        <CreateIdentityForm
          key={request.nonce}
          portfolioId={portfolioId}
          platforms={platforms}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "edit" ? (
        <EditIdentityForm
          key={request.nonce}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "credentials" && credentialPlatform ? (
        <IdentityCredentialsForm
          key={request.nonce}
          platform={credentialPlatform}
          portfolioId={portfolioId}
          item={request.item}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
      {request?.kind === "credentials" && !credentialPlatform ? (
        <p role="alert">
          Platform information is unavailable. Close this dialog and retry loading platforms.
        </p>
      ) : null}
    </Dialog>
  );
}
