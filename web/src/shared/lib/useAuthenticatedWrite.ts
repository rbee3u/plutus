import { executeWrite, SessionExpiredError, type ApiResult } from "@/api";
import { useSessionActions } from "@/shared/lib/session";
import { useCallback } from "react";

export async function authenticatedWrite<T, E = unknown>(
  request: () => Promise<ApiResult<T, E>>,
  expire: () => Promise<void>,
): Promise<ApiResult<T, E>> {
  const result = await executeWrite(request);
  if (result.response?.status === 401) {
    await expire();
    throw new SessionExpiredError();
  }
  return result;
}

/** CSRF and session lifetime only; payload, conflict, and validation stay local. */
export function useAuthenticatedWrite() {
  const { expireSession } = useSessionActions();
  return useCallback(
    <T, E = unknown>(request: () => Promise<ApiResult<T, E>>) =>
      authenticatedWrite(request, expireSession),
    [expireSession],
  );
}
