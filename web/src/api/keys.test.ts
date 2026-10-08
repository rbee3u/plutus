import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";
import { invalidatePortfolioReads, keys } from "./keys";

describe("Collection cache policy", () => {
  it("invalidates dependent reads only within the authenticated Portfolio", async () => {
    const client = new QueryClient();
    const reads = [
      keys.overview("alice", "main"),
      keys.historyWindow("alice", "main", "UTC", "3D", "", "mount", 2),
      keys.accountPositions("alice", "main", "account", "point"),
    ];
    const separate = [
      keys.overview("bob", "main"),
      keys.overview("alice", "other"),
      keys.portfolio("alice", "main"),
      keys.manualEditor("alice", "main", "identity", "editor"),
      keys.job("alice", "main", "job"),
      keys.capitalFlow("alice", "main", "flow"),
      keys.portfolios("alice"),
    ];
    for (const key of [...reads, ...separate]) client.setQueryData(key, { version: 7 });
    await invalidatePortfolioReads(client, "alice", "main");
    for (const key of reads) expect(client.getQueryState(key)?.isInvalidated).toBe(true);
    for (const key of separate) {
      expect(client.getQueryState(key)?.isInvalidated).toBe(false);
      expect(client.getQueryData(key)).toEqual({ version: 7 });
    }
    client.clear();
  });

  it("keeps independent editor instances and history anchors distinct", () => {
    expect(keys.manualEditor("alice", "main", "id", "first")).not.toEqual(
      keys.manualEditor("alice", "main", "id", "second"),
    );
    expect(keys.historyWindow("alice", "main", "UTC", "3D", "", "mount", 1)).not.toEqual(
      keys.historyWindow("alice", "main", "UTC", "3D", "", "mount", 2),
    );
  });
});
