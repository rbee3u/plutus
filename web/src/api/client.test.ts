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
      vi.mocked(getAuthCsrf).mockImplementationOnce(acquire as typeof getAuthCsrf);
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

// These scenarios assert network boundaries and session ownership, not token encoding.
describe("CSRF session cache", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    resetCsrf();
  });
  function acquired(token: string) {
    return {
      data: { token },
      error: undefined,
      request: new Request("http://127.0.0.1/api/v1/auth/csrf"),
      response: new Response(),
    };
  }
  it("shares concurrent acquisition and reuses the token for subsequent writes", async () => {
    let resolve!: (value: ReturnType<typeof acquired>) => void;
    vi.mocked(getAuthCsrf).mockImplementationOnce(
      () =>
        new Promise<ReturnType<typeof acquired>>((done) => {
          resolve = done;
        }),
    );
    const request = vi.fn().mockResolvedValue({ response: new Response(null, { status: 204 }) });
    const first = executeWrite(request);
    const second = executeWrite(request);
    expect(getAuthCsrf).toHaveBeenCalledTimes(1);
    resolve(acquired("one"));
    await Promise.all([first, second]);
    await executeWrite(request);
    expect(getAuthCsrf).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledTimes(3);
  });
  it("refreshes and retries exactly once only for an explicit CSRF rejection", async () => {
    vi.mocked(getAuthCsrf)
      .mockResolvedValueOnce(acquired("old"))
      .mockResolvedValueOnce(acquired("new"));
    const rejected = {
      error: { code: "csrf_invalid" },
      response: new Response(null, { status: 403 }),
    };
    const request = vi.fn().mockResolvedValue(rejected);
    expect(await executeWrite(request)).toBe(rejected);
    expect(getAuthCsrf).toHaveBeenCalledTimes(2);
    expect(request).toHaveBeenCalledTimes(2);
  });
  it.each(["network", "business"])("does not retry a %s failure", async (failure) => {
    vi.mocked(getAuthCsrf).mockResolvedValue(acquired("one"));
    const request = vi.fn().mockImplementation(() =>
      failure === "network"
        ? Promise.reject(new Error("offline"))
        : Promise.resolve({
            error: { code: "forbidden" },
            response: new Response(null, { status: 403 }),
          }),
    );
    if (failure === "network") await expect(executeWrite(request)).rejects.toThrow("offline");
    else await executeWrite(request);
    expect(request).toHaveBeenCalledTimes(1);
    expect(getAuthCsrf).toHaveBeenCalledTimes(1);
  });
  it("does not publish a token or issue a write after its session has been replaced", async () => {
    let resolve!: (value: ReturnType<typeof acquired>) => void;
    vi.mocked(getAuthCsrf).mockImplementationOnce(
      () =>
        new Promise<ReturnType<typeof acquired>>((done) => {
          resolve = done;
        }),
    );
    const oldWrite = vi.fn();
    const pending = executeWrite(oldWrite);
    resetCsrf();
    vi.mocked(getAuthCsrf).mockResolvedValueOnce(acquired("new-session"));
    const currentWrite = vi
      .fn()
      .mockResolvedValue({ response: new Response(null, { status: 204 }) });
    await executeWrite(currentWrite);
    resolve(acquired("obsolete"));
    await expect(pending).rejects.toThrow("session is no longer valid");
    expect(oldWrite).not.toHaveBeenCalled();
    await executeWrite(currentWrite);
    expect(getAuthCsrf).toHaveBeenCalledTimes(2);
  });
});
