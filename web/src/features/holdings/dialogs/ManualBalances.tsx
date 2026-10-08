import { useManual, manualChanges, type ManualTarget } from "./manual";
import { ManualEditor } from "./ManualEditor";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import shared from "@/shared/styles/page.module.css";

export function ManualPending({
  portfolioId,
  identityId,
  name,
  onOpen,
}: {
  portfolioId: string;
  identityId: string;
  name?: string;
  onOpen: (target: ManualTarget) => void;
}) {
  const query = useManual(portfolioId, identityId);
  return (
    <>
      <QueryRecovery query={query} fallback="Manual records could not be loaded." />
      {query.data?.pending ? (
        <div className={shared.notice}>
          <div className={shared.headingRow}>
            <strong>{name ? `${name} · ` : ""}Manual changes awaiting collection</strong>
            <ActionButton tone="ghost" onClick={() => onOpen({ identityId })}>
              Manage records
            </ActionButton>
          </div>
          <DataTable
            caption="Pending manual quantities"
            columns={[
              { key: "symbol", header: "Asset" },
              { key: "oldQuantity", header: "Last collected", numeric: true },
              { key: "quantity", header: "Next collection", numeric: true },
            ]}
            rows={manualChanges(query.data).map((item) => ({ ...item, id: item.symbol }))}
          />
          <p>These changes are excluded from the current Valuation Point.</p>
        </div>
      ) : null}
    </>
  );
}

export function ManualDialog({
  portfolioId,
  target,
  onClose,
}: {
  portfolioId: string;
  target: ManualTarget | null;
  onClose: () => void;
}) {
  return (
    <Dialog open={target !== null} title="Manual balances" onClose={onClose}>
      {target ? (
        <ManualEditor
          key={`${target.identityId}:${target.symbol ?? ""}:${target.action ?? "add"}`}
          portfolioId={portfolioId}
          target={target}
          onClose={onClose}
        />
      ) : null}
    </Dialog>
  );
}
