import { Dialog } from "@/shared/ui/Dialog";
import shared from "@/shared/styles/page.module.css";

export function ExposureHowItWorks({ open, onClose }: { open: boolean; onClose: () => void }) {
  return (
    <Dialog open={open} title="How portfolio exposure works" onClose={onClose}>
      <p>
        Portfolio exposure is the signed coin quantity left after netting each selected account's
        balance equity against its open linear and inverse contracts, then summing every account and
        platform by canonical asset. Long is positive; short is negative.
      </p>
      <p>
        Funding, earn, and dual investment principal are already in balances and are counted once.
        Manual balances are included; pending manual edits and excluded assets are not.
      </p>
      <p>
        A partial row sums the accounts it knows and names the rest. An unknown row is shown as —,
        never zero. A stale report is carried forward and marked.
      </p>
      <p>
        Cross-platform netting is not a liquidation, margin, or hedge measure. Value (USD) uses one
        reference price from the largest absolute known contribution; it is a display reference, not
        a valuation. Options are not collected, and Binance portfolio margin may omit them.
      </p>
      <p className={shared.muted}>
        Pending manual edits take effect at the next collection cycle. Excluded assets keep their
        catalog exclusion and are omitted from this view.
      </p>
    </Dialog>
  );
}
