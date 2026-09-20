import { client } from "@/api/generated/client.gen";
import { getAuthCsrf } from "@/api/generated/sdk.gen";

let csrfToken = "";

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

export async function refreshCsrf(): Promise<string> {
  const result = await getAuthCsrf();
  if (result.error || !result.data) {
    throw new Error("Could not obtain a CSRF token.");
  }
  csrfToken = result.data.token;
  return csrfToken;
}

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
