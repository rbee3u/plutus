import { MoreHorizontal } from "lucide-react";
import type { CapitalFlow } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { Menu, MenuItem, useMenu } from "@/shared/ui/Menu";
import shared from "@/shared/styles/page.module.css";

interface FlowActionsProps {
  item: CapitalFlow;
  disabled?: boolean;
  onEdit: (item: CapitalFlow) => void;
  onDelete: (item: CapitalFlow) => void;
}

export function FlowActions({ item, disabled, onEdit, onDelete }: FlowActionsProps) {
  const menu = useMenu();

  function pick(action: () => void) {
    menu.close(false);
    action();
  }

  return (
    <span className={shared.rowActions}>
      <IconButton
        ref={(node) => menu.setAnchor(node)}
        label={`Actions for flow ${item.canonicalAssetSymbol}`}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        disabled={disabled}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <MoreHorizontal size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <Menu menu={menu} label="Flow actions">
        <MenuItem onClick={() => pick(() => onEdit(item))}>Edit</MenuItem>
        <MenuItem danger onClick={() => pick(() => onDelete(item))}>
          Delete
        </MenuItem>
      </Menu>
    </span>
  );
}
