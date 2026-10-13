import { MoreHorizontal } from "lucide-react";
import type { Asset } from "@/api";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import { Menu, MenuItem, useMenu } from "@/shared/ui/Menu";
import shared from "@/shared/styles/page.module.css";

interface AssetActionsProps {
  item: Asset;
  onMap: () => void;
  onExclude: () => void;
}

export function AssetActions({ item, onMap, onExclude }: AssetActionsProps) {
  const menu = useMenu();

  function pick(action: () => void) {
    menu.close(false);
    action();
  }

  return (
    <span className={shared.rowActions}>
      <IconButton
        ref={(node) => menu.setAnchor(node)}
        label={`Actions for ${item.symbol}`}
        aria-haspopup="menu"
        aria-expanded={menu.open}
        onClick={() => menu.setOpen((open) => !open)}
      >
        <MoreHorizontal size={iconSize.sm} aria-hidden="true" />
      </IconButton>
      <Menu menu={menu} label={`Actions for ${item.symbol}`}>
        <MenuItem onClick={() => pick(onMap)}>Map canonical asset</MenuItem>
        <MenuItem danger={!item.excluded} onClick={() => pick(onExclude)}>
          {item.excluded ? "Restore asset" : "Exclude asset"}
        </MenuItem>
      </Menu>
    </span>
  );
}
