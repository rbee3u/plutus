import { MoreHorizontal } from "lucide-react";
import type { Identity } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
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

  function pick(action: () => void, restoreFocus: boolean) {
    menu.close(restoreFocus);
    action();
  }

  return (
    <span className={shared.rowActions}>
      <IconButton
        ref={(node) => menu.setAnchor(node)}
        label={`Actions for ${item.displayName}`}
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
