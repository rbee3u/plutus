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

export function problemMessage(error: unknown, fallback: string): string {
  if (error && typeof error === "object" && "detail" in error && typeof error.detail === "string") {
    return error.detail;
  }
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}
