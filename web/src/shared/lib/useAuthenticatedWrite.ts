import { executeWrite, SessionExpiredError, type ApiResult } from "@/api";
import { useSessionActions } from "@/shared/lib/session";
import { useCallback } from "react";

/** Only a documented business rejection may use 401 without ending the session. */
export interface WriteOptions {
  acceptUnauthorized?: (error: unknown) => boolean;
}

export async function authenticatedWrite<T>(
  request: () => Promise<ApiResult<T>>,
  expire: () => Promise<void>,
  options?: WriteOptions,
): Promise<ApiResult<T>> {
  const result = await executeWrite(request);
  if (result.response?.status === 401 && !options?.acceptUnauthorized?.(result.error)) {
    await expire();
    throw new SessionExpiredError();
  }
  return result;
}

/** CSRF and session lifetime only; payload, conflict, and validation stay local. */
export function useAuthenticatedWrite() {
  const { expireSession } = useSessionActions();
  return useCallback(
    <T>(request: () => Promise<ApiResult<T>>, options?: WriteOptions) =>
      authenticatedWrite(request, expireSession, options),
    [expireSession],
  );
}
