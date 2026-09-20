import styles from "@/features/pages.module.css";

export interface DataColumn {
  key: string;
  header: string;
  numeric?: boolean;
}

interface DataTableProps {
  caption: string;
  columns: DataColumn[];
  rows: Record<string, string>[];
}

export function DataTable({ caption, columns, rows }: DataTableProps) {
  return (
    <div className={styles.tableWrap}>
      <table>
        <caption>{caption}</caption>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column.key} scope="col">
                {column.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length}>No rows.</td>
            </tr>
          ) : (
            rows.map((row, index) => (
              <tr key={`${row.id ?? row.key ?? index}`}>
                {columns.map((column) => (
                  <td key={column.key} className={column.numeric ? styles.numeric : undefined}>
                    {row[column.key] ?? "—"}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}
