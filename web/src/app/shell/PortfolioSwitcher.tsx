import { ChevronDown } from "lucide-react";
import {
  type CSSProperties,
  type FormEvent,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";
import { useLocation, useNavigate } from "react-router";
import { createPortfolio } from "@/api/generated/sdk.gen";
import type { Portfolio } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import { LAST_PORTFOLIO_KEY } from "@/app/shell/navigation";
import { iconSize } from "@/shared/icons/iconSizes";
import { storePreference } from "@/shared/lib/preferences";
import { ActionButton } from "@/shared/ui/ActionButton";
import { Dialog } from "@/shared/ui/Dialog";
import { TextInput } from "@/shared/ui/FormControls";
import styles from "@/app/shell/Shell.module.css";
import pageStyles from "@/features/pages.module.css";

export type PortfolioOption = Pick<Portfolio, "id" | "name" | "undeletable">;

export function PortfolioSwitcher({
  portfolioId,
  portfolios,
}: {
  portfolioId: string;
  portfolios: PortfolioOption[];
}) {
  const navigate = useNavigate();
  const location = useLocation();
  const [menuOpen, setMenuOpen] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [name, setName] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const createKey = useRef(crypto.randomUUID());
  const rootRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const menuRef = useRef<HTMLUListElement>(null);
  const [menuStyle, setMenuStyle] = useState<CSSProperties>({});
  const labelId = useId();
  const current = portfolios.find((item) => item.id === portfolioId) ?? portfolios[0];

  useLayoutEffect(() => {
    if (!menuOpen || !buttonRef.current) {
      return;
    }
    const update = () => {
      const rect = buttonRef.current?.getBoundingClientRect();
      if (!rect) {
        return;
      }
      setMenuStyle({
        top: rect.bottom + 4,
        right: Math.max(8, window.innerWidth - rect.right),
      });
    };
    update();
    window.addEventListener("resize", update);
    return () => window.removeEventListener("resize", update);
  }, [menuOpen]);

  useEffect(() => {
    if (!menuOpen) {
      return;
    }
    const onPointer = (event: MouseEvent) => {
      const target = event.target as Node;
      if (buttonRef.current?.contains(target) || menuRef.current?.contains(target)) {
        return;
      }
      setMenuOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMenuOpen(false);
      }
    };
    document.addEventListener("mousedown", onPointer);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onPointer);
      document.removeEventListener("keydown", onKey);
    };
  }, [menuOpen]);

  function selectPortfolio(next: string) {
    storePreference(LAST_PORTFOLIO_KEY, next);
    setMenuOpen(false);
    const suffix = location.pathname.replace(/^\/p\/[^/]+/, "") || "/overview";
    void navigate(`/p/${next}${suffix}`);
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
      createKey.current = crypto.randomUUID();
      setName("");
      setCreateOpen(false);
      storePreference(LAST_PORTFOLIO_KEY, result.data.id);
      const suffix = location.pathname.replace(/^\/p\/[^/]+/, "") || "/overview";
      void navigate(`/p/${result.data.id}${suffix}`);
    } catch (cause) {
      setError(
        problemMessage(cause, "The portfolio could not be created. Check the network and retry."),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className={styles.switcher} ref={rootRef}>
      <span id={labelId} className={styles.switcherLabel}>
        Portfolio
      </span>
      <div className={styles.switcherControls}>
        <button
          ref={buttonRef}
          type="button"
          className={styles.switcherButton}
          aria-labelledby={labelId}
          aria-haspopup="listbox"
          aria-expanded={menuOpen}
          onClick={() => setMenuOpen((open) => !open)}
        >
          <span className={styles.switcherName}>{current?.name ?? "Portfolio"}</span>
          {current?.undeletable ? (
            <span className={styles.undeletableBadge}>undeletable</span>
          ) : null}
          <ChevronDown size={iconSize.xs} aria-hidden="true" />
        </button>
        {menuOpen
          ? createPortal(
              <ul
                ref={menuRef}
                className={styles.switcherMenu}
                role="listbox"
                aria-labelledby={labelId}
                style={menuStyle}
              >
                {portfolios.map((item) => (
                  <li key={item.id}>
                    <button
                      type="button"
                      role="option"
                      aria-selected={item.id === portfolioId}
                      className={styles.switcherOption}
                      onClick={() => selectPortfolio(item.id)}
                    >
                      <span className={styles.switcherName}>{item.name}</span>
                      {item.undeletable ? (
                        <span className={styles.undeletableBadge}>undeletable</span>
                      ) : null}
                    </button>
                  </li>
                ))}
              </ul>,
              document.body,
            )
          : null}
        <ActionButton
          type="button"
          onClick={() => {
            setMenuOpen(false);
            setCreateOpen(true);
          }}
        >
          New
        </ActionButton>
      </div>
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
            Name the workspace. Timezone and display Asset start as Asia/Shanghai and USD.
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
    </div>
  );
}
