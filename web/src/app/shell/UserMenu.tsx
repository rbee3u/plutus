import { ChevronDown, LogOut, ShieldCheck, User } from "lucide-react";
import { useCurrentUser } from "@/shared/lib/session";
import { iconSize } from "@/shared/icons/iconSizes";
import { Menu, MenuItem, MenuSeparator, useMenu } from "@/shared/ui/Menu";
import styles from "@/app/shell/Shell.module.css";

export function UserMenu({
  portfolioId,
  onSignOut,
}: {
  portfolioId?: string;
  onSignOut: () => void;
}) {
  const user = useCurrentUser();
  const menu = useMenu();
  const username = user.data?.username ?? "Account";
  const securityPath = portfolioId ? `/p/${portfolioId}/settings/security` : "/account/security";

  return (
    <>
      <button
        ref={(node) => menu.setAnchor(node)}
        type="button"
        className={styles.userButton}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        onClick={() => menu.setOpen((current) => !current)}
      >
        <User size={iconSize.xs} aria-hidden="true" />
        <span className={styles.switcherName}>{username}</span>
        <ChevronDown size={iconSize.xs} aria-hidden="true" />
      </button>
      <Menu menu={menu} label="Account">
        <MenuItem
          to={securityPath}
          icon={<ShieldCheck size={iconSize.sm} />}
          onClick={() => menu.close()}
        >
          Account & security
        </MenuItem>
        <MenuSeparator />
        <MenuItem
          icon={<LogOut size={iconSize.sm} />}
          onClick={() => {
            menu.close();
            onSignOut();
          }}
        >
          Sign out
        </MenuItem>
      </Menu>
    </>
  );
}
