import { Dialog } from "@/shared/ui/Dialog";
import shared from "@/shared/styles/page.module.css";

export function ExposureHowItWorks({
  open,
  onClose,
  stablecoinSymbols,
}: {
  open: boolean;
  onClose: () => void;
  stablecoinSymbols: string[];
}) {
  const classified = stablecoinSymbols.length > 0 ? stablecoinSymbols.join(", ") : "none";
  return (
    <Dialog open={open} title="How portfolio exposure works" onClose={onClose}>
      <p>
        Portfolio exposure is the signed coin quantity left after netting each selected account's
        balance equity against its open linear and inverse contracts, then summing every account and
        platform by canonical asset. Long is positive; short is negative.
      </p>
      <p>
        The three USD amounts are a signed display composition: each row's net quantity times its
        reference price, grouped into stablecoins, non-stablecoins, and total. Long and short values
        offset. They are not a valuation, net worth, hedge, or liquidation measure.
      </p>
      <p>
        Stablecoins are USD-pegged canonical assets in the maintained set. This view classifies:{" "}
        {classified}. USD, USDT, and USDC value at one USD; other stablecoins use their reference
        price.
      </p>
      <p>
        A row without a USD value is left out of the amounts and named in the summary note. A class
        with no known value shows —, never $0; a known zero shows $0. Missing accounts and excluded
        assets are named in the coverage notice.
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
