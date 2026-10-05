import { expect, it } from "vitest";
import { safeJson } from "./json";

it("redacts sensitive fields recursively while preserving ordinary data", () => {
  const output = safeJson({
    symbol: "BTC",
    apiKey: "top-secret",
    accounts: [
      { name: "trading", credentials: { Password: "hunter2", nonce: "123" } },
      { name: "funding", balance: "12.34" },
    ],
  });

  expect(JSON.parse(output)).toEqual({
    symbol: "BTC",
    apiKey: "[redacted]",
    accounts: [
      { name: "trading", credentials: { Password: "[redacted]", nonce: "[redacted]" } },
      { name: "funding", balance: "12.34" },
    ],
  });
  expect(output).not.toContain("top-secret");
  expect(output).not.toContain("hunter2");
});
