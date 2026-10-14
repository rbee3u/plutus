import type { ReactNode } from "react";
import shared from "@/shared/styles/page.module.css";

export interface TableEmptyStateProps {
  title: string;
  help?: ReactNode;
  action?: ReactNode;
}

export function TableEmptyState({ title, help, action }: TableEmptyStateProps) {
  return (
    <div className={shared.emptyState}>
      <p className={shared.emptyStateTitle}>{title}</p>
      {help ? <p className={shared.emptyStateHelp}>{help}</p> : null}
      {action ? <div className={shared.emptyStateAction}>{action}</div> : null}
    </div>
  );
}
