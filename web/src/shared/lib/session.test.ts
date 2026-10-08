import { getAuthMe, keys, SessionExpiredError, unwrapOrRedirect, type User } from "@/api";
import { QueryClient } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  clearSession,
  currentUserOptions,
  endSession,
  expireSession,
  sessionGeneration,
} from "./session";

vi.mock("@/api", async (original) => ({
  ...(await original<typeof import("@/api")>()),
  getAuthMe: vi.fn(),
}));

const user: User = { id: "alice", username: "alice", status: "active" };

describe("Authenticated cache lifetime", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shares one cached User across independent consumers", async () => {
    vi.mocked(getAuthMe).mockResolvedValue({
      data: { user, sessionCount: 1 },
      error: undefined,
      response: new Response(),
    });
    const client = new QueryClient();
    expect(await client.fetchQuery(currentUserOptions())).toEqual(user);
    expect(await client.fetchQuery(currentUserOptions())).toEqual(user);
    expect(getAuthMe).toHaveBeenCalledTimes(1);
    client.clear();
  });

  it("distinguishes unauthenticated state from a transport failure", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(getAuthMe).mockResolvedValueOnce({
      data: undefined,
      error: {
        type: "about:blank",
        title: "Unauthorized",
        status: 401,
        detail: "Session expired",
        code: "unauthenticated",
      },
      response: new Response(null, { status: 401 }),
    });
    expect(await client.fetchQuery({ ...currentUserOptions(), retry: false })).toBeNull();
    client.clear();
    vi.mocked(getAuthMe).mockRejectedValueOnce(new Error("offline"));
    await expect(client.fetchQuery({ ...currentUserOptions(), retry: false })).rejects.toThrow(
      "offline",
    );
    client.clear();
  });

  it("cancels in-flight reads before another User can acquire cached data", async () => {
    const client = new QueryClient();
    client.setQueryData(keys.authMe(), user);
    client.setQueryData(keys.manualEditor("alice", "main", "id", "editor"), { version: 7 });
    let finish: (value: string) => void = () => {};
    const pending = client
      .fetchQuery({
        queryKey: keys.overview("alice", "main"),
        queryFn: () =>
          new Promise<string>((resolve) => {
            finish = resolve;
          }),
      })
      .catch(() => undefined);
    await clearSession(client);
    finish("old data");
    await pending;
    expect(client.getQueryData(keys.authMe())).toBeNull();
    expect(client.getQueryData(keys.overview("alice", "main"))).toBeUndefined();
    expect(client.getQueryData(keys.manualEditor("alice", "main", "id", "editor"))).toBeUndefined();
    client.clear();
  });

  it("preserves the password-success notice when ending the session", async () => {
    const client = new QueryClient();
    const navigate = vi.fn();
    await endSession(client, navigate, { notice: "Password changed." });
    expect(navigate).toHaveBeenCalledWith("/login", {
      replace: true,
      state: { notice: "Password changed." },
    });
    client.clear();
  });

  it.each(["alice", "bob"])("ignores old read expiry after signing in again as %s", async (id) => {
    const client = new QueryClient();
    client.setQueryData(keys.authMe(), user);
    const navigate = vi.fn();
    const generation = sessionGeneration(client);
    let finish: (value: { response: Response }) => void = () => {};
    const response = new Promise<{ response: Response }>((resolve) => {
      finish = resolve;
    });
    let expiry: Promise<void> | undefined;
    const pending = response
      .then((result) =>
        unwrapOrRedirect(result, () => {
          expiry = expireSession(client, generation, navigate);
        }),
      )
      .catch((error: unknown) => error);
    await clearSession(client);
    const replacement = { ...user, id };
    client.setQueryData(keys.authMe(), replacement);
    client.setQueryData(keys.overview(id, "main"), "new data");
    finish({ response: new Response(null, { status: 401 }) });
    expect(await pending).toBeInstanceOf(SessionExpiredError);
    await expiry;
    expect(client.getQueryData(keys.authMe())).toEqual(replacement);
    expect(client.getQueryData(keys.overview(id, "main"))).toBe("new data");
    expect(navigate).not.toHaveBeenCalled();
    client.clear();
  });
});
