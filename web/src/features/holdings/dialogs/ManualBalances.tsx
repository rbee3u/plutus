import type { ManualTarget } from "./manual";
import { ManualEditor } from "./ManualEditor";
import { Dialog } from "@/shared/ui/Dialog";

export function ManualDialog({
  portfolioId,
  target,
  onClose,
  onSaved,
}: {
  portfolioId: string;
  target: ManualTarget | null;
  onClose: () => void;
  onSaved?: () => void;
}) {
  return (
    <Dialog open={target !== null} title="Manual balances" onClose={onClose}>
      {target ? (
        <ManualEditor
          key={`${target.identityId}:${target.symbol ?? ""}:${target.action ?? "add"}`}
          portfolioId={portfolioId}
          target={target}
          onClose={onClose}
          onSaved={onSaved}
        />
      ) : null}
    </Dialog>
  );
}
