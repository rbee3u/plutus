import { ChevronDown, Plus } from "lucide-react";
import { type FormEvent, useRef, useState } from "react";
import { useLocation, useNavigate } from "react-router";
import { createPortfolio } from "@/api/generated/sdk.gen";
import type { Portfolio } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY, isAccountPath } from "@/app/shell/navigation";
import { iconSize } from "@/shared/icons/iconSizes";
import { newIdempotencyKey } from "@/shared/lib/id";
import { storePreference } from "@/shared/lib/preferences";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import { Menu, MenuItem, MenuSeparator, useMenu } from "@/shared/ui/Menu";
import styles from "@/app/shell/Shell.module.css";
import pageStyles from "@/features/pages.module.css";

export type PortfolioOption = Pick<Portfolio, "id" | "name">;

export function PortfolioSwitcher({
  portfolioId,
  portfolios,
  onPortfolioRemembered,
}: {
  portfolioId: string;
  portfolios: PortfolioOption[];
  onPortfolioRemembered?: (id: string) => void;
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const menu = useMenu();
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const createKey = useRef(newIdempotencyKey());
  const current = portfolios.find((item) => item.id === portfolioId) ?? portfolios[0];

  function pathFor(id: string): string {
    if (isAccountPath(location.pathname)) {
      return `/p/${id}/overview`;
    }
    const suffix = location.pathname.replace(/^\/p\/[^/]+/, "") || "/overview";
    return `/p/${id}${suffix}`;
  }

  function selectPortfolio(next: string) {
    storePreference(LAST_PORTFOLIO_KEY, next);
    menu.close(true);
    if (isAccountPath(location.pathname)) {
      onPortfolioRemembered?.(next);
      return;
    }
    void navigate(pathFor(next));
  }

  async function onCreate(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await refreshCsrf();
      const result = await createPortfolio({
        headers: { "Idempotency-Key": createKey.current },
        body: { name },
      });
      if (result.response?.status === 401) {
        void navigate("/login", { replace: true });
        return;
      }
      if (result.error || !result.data) {
        setError(problemMessage(result.error, "The portfolio could not be created."));
        return;
      }
      createKey.current = newIdempotencyKey();
      setName("");
      setCreateOpen(false);
      storePreference(LAST_PORTFOLIO_KEY, result.data.id);
      void navigate(pathFor(result.data.id));
    } catch (cause) {
      setError(
        problemMessage(cause, "The portfolio could not be created. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <>
      <button
        ref={(node) => menu.setAnchor(node)}
        type="button"
        className={styles.switcherButton}
        title="Switch portfolio"
        aria-haspopup="menu"
        aria-expanded={menu.open}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <span className={styles.switcherName}>{current?.name ?? "Portfolio"}</span>
        <ChevronDown size={iconSize.xs} aria-hidden="true" />
      </button>
      <Menu menu={menu} label="Portfolios">
        {portfolios.map((item) => (
          <MenuItem
            key={item.id}
            role="menuitemradio"
            checked={item.id === current?.id}
            onClick={() => selectPortfolio(item.id)}
          >
            {item.name}
          </MenuItem>
        ))}
        <MenuSeparator />
        <MenuItem
          icon={<Plus size={iconSize.sm} />}
          onClick={() => {
            menu.close(true);
            setCreateOpen(true);
          }}
        >
          New portfolio
        </MenuItem>
      </Menu>
      <Dialog
        open={createOpen}
        title="New portfolio"
        onClose={() => {
          if (!busy) {
            setCreateOpen(false);
            setError(null);
          }
        }}
      >
        <form className={pageStyles.form} onSubmit={(event) => void onCreate(event)}>
          <p className={pageStyles.lead}>
            Valuation is USD-only. The timezone and collection interval can be changed later in
            Settings.
          </p>
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
                setCreateOpen(false);
                setError(null);
              }}
            >
              Cancel
            </ActionButton>
          </div>
        </form>
      </Dialog>
    </>
  );
}
