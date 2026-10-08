import { client } from "./generated/client.gen";
import { getAuthCsrf } from "./generated/sdk.gen";
import { configureApiClient, executeWrite, refreshCsrf, resetCsrf } from "./client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("./generated/sdk.gen", () => ({ getAuthCsrf: vi.fn() }));

describe("Write transport", () => {
  beforeAll(() => configureApiClient());
  beforeEach(() => {
    vi.clearAllMocks();
    resetCsrf();
  });
  afterEach(() => vi.unstubAllGlobals());

  it("acquires CSRF before invoking a write and preserves a no-content response", async () => {
    const order: string[] = [];
    vi.mocked(getAuthCsrf).mockImplementation(() => {
      order.push("csrf");
      return Promise.resolve({
        data: { token: "token" },
        error: undefined,
        response: new Response(),
      });
    });
    const result = { response: new Response(null, { status: 204 }) };
    expect(
      await executeWrite(() => {
        order.push("write");
        return Promise.resolve(result);
      }),
    ).toBe(result);
    expect(order).toEqual(["csrf", "write"]);
  });

  it.each([
    {
      name: "rejected request",
      acquire: () => Promise.reject(new Error("offline")),
      message: "offline",
    },
    {
      name: "API error",
      acquire: () =>
        Promise.resolve({
          data: undefined,
          error: { detail: "Unavailable" },
          response: new Response(null, { status: 503 }),
        }),
      message: "Could not obtain a CSRF token.",
    },
    {
      name: "missing payload",
      acquire: () =>
        Promise.resolve({
          data: undefined,
          error: undefined,
          response: new Response(null, { status: 204 }),
        }),
      message: "Could not obtain a CSRF token.",
    },
  ])(
    "does not issue a write after CSRF acquisition returns $name",
    async ({ acquire, message }) => {
      vi.mocked(getAuthCsrf).mockImplementationOnce(acquire);
      const request = vi.fn();
      await expect(executeWrite(request)).rejects.toThrow(message);
      expect(request).not.toHaveBeenCalled();
    },
  );

  it("clears the outgoing CSRF header when the session ends", async () => {
    vi.mocked(getAuthCsrf).mockResolvedValue({
      data: { token: "old-token" },
      error: undefined,
      response: new Response(),
    });
    const fetch = vi
      .fn()
      .mockImplementation(() => Promise.resolve(new Response(null, { status: 204 })));
    vi.stubGlobal("fetch", fetch);
    client.setConfig({ fetch, baseUrl: "http://127.0.0.1" });
    await refreshCsrf();
    const result = await client.post({ url: "/write" });
    expect(result.error).toBeUndefined();
    const before = fetch.mock.calls[0]?.[0] as Request;
    expect(before.headers.get("X-CSRF-Token")).toBe("old-token");
    resetCsrf();
    await client.post({ url: "/write" });
    const after = fetch.mock.calls[1]?.[0] as Request;
    expect(after.headers.has("X-CSRF-Token")).toBe(false);
  });
});
