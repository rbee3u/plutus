import { X } from "lucide-react";
import { type ReactNode, useEffect, useId, useRef } from "react";
import { iconSize } from "@/shared/icons/iconSizes";
import { IconButton } from "@/shared/ui/IconButton";
import styles from "@/shared/ui/Dialog.module.css";

export function Dialog({
  open,
  title,
  onClose,
  children,
}: {
  open: boolean;
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const dialogRef = useRef<HTMLDialogElement>(null);
  const restoreRef = useRef<HTMLElement | null>(null);
  const titleId = useId();
  const bodyId = useId();

  useEffect(() => {
    const node = dialogRef.current;
    if (!node) {
      return;
    }
    if (open) {
      if (document.activeElement instanceof HTMLElement) {
        restoreRef.current = document.activeElement;
      }
      if (!node.open) {
        node.showModal();
      }
      const focusable = node.querySelector<HTMLElement>(
        `[data-dialog-body] button, [data-dialog-body] [href], [data-dialog-body] input, [data-dialog-body] select, [data-dialog-body] textarea, [data-dialog-body] [tabindex]:not([tabindex="-1"])`,
      );
      (focusable ?? node.querySelector<HTMLElement>("button"))?.focus();
      return;
    }
    if (node.open) {
      node.close();
    }
  }, [open]);

  return (
    <dialog
      ref={dialogRef}
      className={styles.dialog}
      aria-labelledby={titleId}
      aria-describedby={bodyId}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
      onClose={() => {
        restoreRef.current?.focus();
        restoreRef.current = null;
        if (open) {
          onClose();
        }
      }}
      onClick={(event) => {
        if (event.target === dialogRef.current) {
          onClose();
        }
      }}
    >
      <header className={styles.header}>
        <h2 id={titleId}>{title}</h2>
        <IconButton label="Close dialog" onClick={onClose}>
          <X size={iconSize.md} aria-hidden="true" />
        </IconButton>
      </header>
      <div id={bodyId} className={styles.body} data-dialog-body="true">
        {children}
      </div>
    </dialog>
  );
}
