import { executeWrite, SessionExpiredError } from "@/api";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { authenticatedWrite } from "./useAuthenticatedWrite";
import { QueryClient } from "@tanstack/react-query";
import { keys } from "@/api";
import { clearSession, expireSession, sessionGeneration } from "./session";

vi.mock("@/api", async (original) => ({
  ...(await original<typeof import("@/api")>()),
  executeWrite: vi.fn(),
}));

describe("Authenticated write boundary", () => {
  beforeEach(() => {
    vi.mocked(executeWrite).mockImplementation((request) => request());
  });

  it.each([204, 412, 422])("preserves status %s and its raw business result", async (status) => {
    const result = {
      response: new Response(null, { status }),
      error: status === 422 ? { ok: false, identity: { id: "id" } } : undefined,
    };
    const expire = vi.fn();
    expect(await authenticatedWrite(() => Promise.resolve(result), expire)).toBe(result);
    expect(expire).not.toHaveBeenCalled();
  });

  it("ends an expired session before surfacing an unauthorized write", async () => {
    const expire = vi.fn().mockResolvedValue(undefined);
    await expect(
      authenticatedWrite(
        () => Promise.resolve({ response: new Response(null, { status: 401 }) }),
        expire,
      ),
    ).rejects.toBeInstanceOf(SessionExpiredError);
    expect(expire).toHaveBeenCalledTimes(1);
  });

  it("preserves a password business rejection without ending the session", async () => {
    const expire = vi.fn();
    const result = {
      error: { code: "invalid_current_password" },
      response: new Response(null, { status: 422 }),
    };
    expect(await authenticatedWrite(() => Promise.resolve(result), expire)).toBe(result);
    expect(expire).not.toHaveBeenCalled();
  });

  it("does not run the SDK write when CSRF acquisition fails", async () => {
    vi.mocked(executeWrite).mockRejectedValueOnce(new Error("CSRF unavailable"));
    const request = vi.fn();
    const expire = vi.fn();
    await expect(authenticatedWrite(request, expire)).rejects.toThrow("CSRF unavailable");
    expect(request).not.toHaveBeenCalled();
    expect(expire).not.toHaveBeenCalled();
  });

  it("retains a same-user replacement session when an old write returns 401", async () => {
    const client = new QueryClient();
    const user = { id: "alice", username: "alice", status: "active" };
    client.setQueryData(keys.authMe(), user);
    const navigate = vi.fn();
    const generation = sessionGeneration(client);
    let finish: (value: { response: Response }) => void = () => {};
    const response = new Promise<{ response: Response }>((resolve) => {
      finish = resolve;
    });
    const pending = authenticatedWrite(
      () => response,
      () => expireSession(client, generation, navigate),
    ).catch((error: unknown) => error);
    await clearSession(client);
    client.setQueryData(keys.authMe(), user);
    finish({ response: new Response(null, { status: 401 }) });
    expect(await pending).toBeInstanceOf(SessionExpiredError);
    expect(client.getQueryData(keys.authMe())).toEqual(user);
    expect(navigate).not.toHaveBeenCalled();
    client.clear();
  });
});
