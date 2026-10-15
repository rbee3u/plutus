import { getAuthSessions, keys, SessionExpiredError, type SessionList } from "@/api";
import { clearSession } from "@/shared/lib/session";
import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { sessionsOptions } from "./useSessions";

vi.mock("@/api", async (original) => ({
  ...(await original<typeof import("@/api")>()),
  getAuthSessions: vi.fn(),
}));

afterEach(() => vi.clearAllMocks());

describe("Security session data", () => {
  it("does not retry failed session reads or treat failures as an empty list", async () => {
    const client = new QueryClient();
    const redirect = vi.fn();
    const onError = vi.fn();
    vi.mocked(getAuthSessions).mockRejectedValue(new Error("offline"));
    await expect(client.fetchQuery(sessionsOptions("alice", redirect, onError))).rejects.toThrow(
      "offline",
    );
    expect(getAuthSessions).toHaveBeenCalledTimes(1);
    expect(onError).toHaveBeenCalledWith("offline");
    expect(client.getQueryData(keys.authSessions("alice"))).toBeUndefined();
    expect(redirect).not.toHaveBeenCalled();
    client.clear();
  });

  it("routes a 401 through shared session expiry", async () => {
    const client = new QueryClient();
    const redirect = vi.fn();
    const onError = vi.fn();
    vi.mocked(getAuthSessions).mockResolvedValue({
      data: undefined,
      error: {
        type: "about:blank",
        title: "Unauthorized",
        status: 401,
        detail: "Expired",
        code: "Unauthenticated.",
      },
      response: new Response(null, { status: 401 }),
    });
    await expect(
      client.fetchQuery(sessionsOptions("alice", redirect, onError)),
    ).rejects.toBeInstanceOf(SessionExpiredError);
    expect(redirect).toHaveBeenCalledOnce();
    expect(onError).not.toHaveBeenCalled();
    client.clear();
  });

  it("cancels and discards in-flight session data when authentication ends", async () => {
    const client = new QueryClient();
    let finish!: (value: { data: SessionList; error: undefined; response: Response }) => void;
    vi.mocked(getAuthSessions).mockImplementation(
      () =>
        new Promise<{ data: SessionList; error: undefined; response: Response }>((resolve) => {
          finish = resolve;
        }),
    );
    const pending = client
      .fetchQuery(sessionsOptions("alice", vi.fn(), vi.fn()))
      .catch(() => undefined);
    await clearSession(client);
    finish({
      data: {
        items: [
          {
            id: "old",
            current: true,
            createdAt: "2026-10-08T00:00:00Z",
            lastSeenAt: "2026-10-08T00:00:00Z",
            idleExpiresAt: "2026-10-09T00:00:00Z",
            absoluteExpiresAt: "2026-10-10T00:00:00Z",
          },
        ],
      },
      error: undefined,
      response: new Response(),
    });
    await pending;
    expect(client.getQueryData(keys.authSessions("alice"))).toBeUndefined();
    expect(client.getQueryData(keys.authSessions("bob"))).toBeUndefined();
    client.clear();
  });
});
