import { MoreHorizontal } from "lucide-react";
import type { Identity } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { ActionButton } from "@/shared/ui/ActionButton";
import { IconButton } from "@/shared/ui/IconButton";
import { Menu, MenuItem, MenuSeparator, useMenu } from "@/shared/ui/Menu";
import shared from "@/shared/styles/page.module.css";

interface IdentityActionsProps {
  item: Identity;
  busy: boolean;
  onEdit: () => void;
  onCredentials: () => void;
  onValidate: () => void;
  onEnable: () => void;
  onDisable: () => void;
  onArchive: () => void;
}

/**
 * The next step for the connection sits in the row; everything else waits in
 * a menu. An enabled connection needs nothing, so its row shows only the menu.
 */
export function IdentityActions({
  item,
  busy,
  onEdit,
  onCredentials,
  onValidate,
  onEnable,
  onDisable,
  onArchive,
}: IdentityActionsProps) {
  const menu = useMenu();
  const connection = item.connection;
  const next = !connection.credentialsConfigured
    ? { label: "Add credentials", run: onCredentials }
    : connection.status === "invalid"
      ? { label: "Validate", run: onValidate }
      : connection.status === "enabled"
        ? null
        : { label: "Enable", run: onEnable };

  function pick(action: () => void, restoreFocus: boolean) {
    menu.close(restoreFocus);
    action();
  }

  return (
    <span className={shared.rowActions}>
      {next ? (
        <ActionButton type="button" tone="secondary" disabled={busy} onClick={next.run}>
          {next.label}
        </ActionButton>
      ) : null}
      <IconButton
        ref={(node) => menu.setAnchor(node)}
        label={`More actions for ${item.displayName}`}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        disabled={busy}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <MoreHorizontal size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <Menu menu={menu} label={`Actions for ${item.displayName}`}>
        <MenuItem onClick={() => pick(onEdit, false)}>Edit</MenuItem>
        <MenuItem onClick={() => pick(onCredentials, false)}>Credentials</MenuItem>
        <MenuItem
          disabled={!connection.credentialsConfigured}
          onClick={() => pick(onValidate, true)}
        >
          Validate
        </MenuItem>
        {connection.status === "enabled" ? (
          <MenuItem onClick={() => pick(onDisable, true)}>Disable</MenuItem>
        ) : (
          <MenuItem
            disabled={!connection.credentialsConfigured}
            onClick={() => pick(onEnable, true)}
          >
            Enable
          </MenuItem>
        )}
        <MenuSeparator />
        <MenuItem
          danger
          disabled={connection.status === "enabled"}
          title={
            connection.status === "enabled" ? "Disable connection before archiving" : undefined
          }
          onClick={() => pick(onArchive, false)}
        >
          Archive
        </MenuItem>
      </Menu>
    </span>
  );
}
