import { Fragment, type MouseEvent, type ReactNode } from "react";
import { NumericText } from "./NumericText";
import styles from "@/shared/styles/page.module.css";

export interface DataColumn {
  key: string;
  header: string;
  /** Optional controls; header remains the plain column name. */
  headerContent?: ReactNode;
  numeric?: boolean;
  /** Keeps timestamps and short codes on one line; the wrapper scrolls instead. */
  nowrap?: boolean;
}

function cellClass(column: DataColumn): string | undefined {
  const classes = [column.numeric ? styles.numeric : "", column.nowrap ? styles.nowrap : ""].filter(
    Boolean,
  );
  return classes.length > 0 ? classes.join(" ") : undefined;
}

/** One row; `id` or `key` (when a string) becomes the React key. */
export type DataRow = Record<string, ReactNode>;

export interface DataTableProps {
  /** Accessible name of the table; read by screen readers, not drawn. */
  caption: string;
  columns: DataColumn[];
  rows: DataRow[];
  /** Sentence shown when there are no rows, saying what would make rows appear. */
  empty?: string;
  rowClassName?: (row: DataRow) => string | undefined;
  /** Key of the row whose detail is open; `renderDetail` fills a full-width row beneath it. */
  openKey?: string;
  /** Element id of the open detail row, for the toggle's `aria-controls`. */
  detailId?: string;
  renderDetail?: (row: DataRow) => ReactNode;
  /** Makes the whole row a target for opening its detail, called with the row key; controls inside the row keep their own click. */
  onRowClick?: (key: string) => void;
}

/** True when a click landed on a control that handles itself, so a row handler should stay quiet. */
export function isOwnControl(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    target.closest("button, a, input, select, textarea, label") !== null
  );
}

function rowKey(row: DataRow, index: number): string {
  const candidate = row.id ?? row.key;
  return typeof candidate === "string" || typeof candidate === "number"
    ? String(candidate)
    : String(index);
}

export function DataTable({
  caption,
  columns,
  rows,
  empty = "No rows.",
  rowClassName,
  openKey,
  detailId,
  renderDetail,
  onRowClick,
}: DataTableProps) {
  function rowClick(key: string) {
    return onRowClick
      ? (event: MouseEvent<HTMLTableRowElement>) => {
          if (!isOwnControl(event.target)) {
            onRowClick(key);
          }
        }
      : undefined;
  }
  function rowClasses(row: DataRow): string | undefined {
    const classes = [onRowClick ? styles.clickRow : "", rowClassName?.(row) ?? ""].filter(Boolean);
    return classes.length > 0 ? classes.join(" ") : undefined;
  }
  return (
    <div className={styles.tableWrap}>
      <table>
        <caption className={styles.srOnly}>{caption}</caption>
        <thead>
          <tr>
            {columns.map((column) => (
              <th
                key={column.key}
                scope="col"
                className={cellClass(column)}
                aria-label={column.headerContent ? column.header : undefined}
              >
                {column.headerContent ?? column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className={styles.cellMeta}>
                {empty}
              </td>
            </tr>
          ) : (
            rows.map((row, index) => {
              const key = rowKey(row, index);
              return (
                <Fragment key={key}>
                  <tr className={rowClasses(row)} onClick={rowClick(key)}>
                    {columns.map((column) => (
                      <td key={column.key} className={cellClass(column)}>
                        {column.numeric && typeof row[column.key] === "string" ? (
                          <NumericText>{row[column.key] as string}</NumericText>
                        ) : (
                          (row[column.key] ?? "—")
                        )}
                      </td>
                    ))}
                  </tr>
                  {renderDetail && openKey === key ? (
                    <tr className={styles.detailRow} id={detailId}>
                      <td colSpan={columns.length}>
                        <div className={styles.detail}>{renderDetail(row)}</div>
                      </td>
                    </tr>
                  ) : null}
                </Fragment>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}
