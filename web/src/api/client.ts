import { client } from "@/api/generated/client.gen";
import { getAuthCsrf } from "@/api/generated/sdk.gen";

let csrfToken = "";

/** End the in-memory token lifetime with the authenticated session. */
export function resetCsrf(): void {
  csrfToken = "";
}

/** Point the generated client at the same origin and attach CSRF headers. */
export function configureApiClient(): void {
  client.setConfig({
    baseUrl: "",
    credentials: "include",
  });
  client.interceptors.request.use((request) => {
    if (request.method && request.method !== "GET" && request.method !== "HEAD" && csrfToken) {
      request.headers.set("X-CSRF-Token", csrfToken);
    }
    return request;
  });
}

/** Read and remember the double-submit CSRF token before a state change. */
export async function refreshCsrf(): Promise<string> {
  const result = await getAuthCsrf();
  if (result.error || !result.data) {
    throw new Error("Could not obtain a CSRF token.");
  }
  csrfToken = result.data.token;
  return csrfToken;
}

/** An RFC 9457 problem document surfaced as an Error. */
export class ProblemError extends Error {
  readonly status?: number;
  readonly code?: string;
  readonly traceId?: string;

  constructor(message: string, init?: { status?: number; code?: string; traceId?: string }) {
    super(message);
    this.name = "ProblemError";
    this.status = init?.status;
    this.code = init?.code;
    this.traceId = init?.traceId;
  }
}

export function problemError(error: unknown, fallback: string): ProblemError {
  if (error instanceof ProblemError) {
    return error;
  }
  if (error && typeof error === "object") {
    const record = error as Record<string, unknown>;
    if (typeof record.detail === "string") {
      return new ProblemError(record.detail, {
        status: typeof record.status === "number" ? record.status : undefined,
        code: typeof record.code === "string" ? record.code : undefined,
        traceId: typeof record.traceId === "string" ? record.traceId : undefined,
      });
    }
  }
  if (error instanceof Error && error.message) {
    return new ProblemError(error.message);
  }
  return new ProblemError(fallback);
}

export function problemMessage(error: unknown, fallback: string): string {
  return problemError(error, fallback).message;
}

/** The shape every generated SDK call returns. */
export interface ApiResult<T> {
  data?: T;
  error?: unknown;
  response?: Response;
}

/** Common write transport; business response interpretation stays with callers. */
export async function executeWrite<T>(request: () => Promise<ApiResult<T>>): Promise<ApiResult<T>> {
  await refreshCsrf();
  return request();
}

export class SessionExpiredError extends Error {
  constructor() {
    super("The session is no longer valid.");
    this.name = "SessionExpiredError";
  }
}

/** Return the payload or throw; a 401 becomes SessionExpiredError. */
export function unwrapData<T>(result: ApiResult<T>): T {
  if (result.response?.status === 401) {
    throw new SessionExpiredError();
  }
  if (result.data === undefined) {
    throw problemError(result.error, "The request could not be completed.");
  }
  return result.data;
}

/** Unwrap, sending an expired session to the login redirect. */
export function unwrapOrRedirect<T>(result: ApiResult<T>, redirect: () => void): T {
  try {
    return unwrapData(result);
  } catch (error) {
    if (error instanceof SessionExpiredError) {
      redirect();
    }
    throw error;
  }
}

export function readEtag(response: Response | undefined, fallback: string): string {
  return response?.headers.get("ETag") ?? fallback;
}
