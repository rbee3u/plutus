import { getAuthMe } from "@/api/generated/sdk.gen";
import type { User } from "@/api/generated/types.gen";
import { problemError } from "@/api/http";
import { useQuery } from "@tanstack/react-query";
import { useNavigate } from "react-router";

export class SessionExpiredError extends Error {
  constructor() {
    super("The session is no longer valid.");
    this.name = "SessionExpiredError";
  }
}

export interface ApiResult<T> {
  data?: T;
  error?: unknown;
  response?: Response;
}

export function unwrapData<T>(result: ApiResult<T>): T {
  if (result.response?.status === 401) {
    throw new SessionExpiredError();
  }
  if (result.data === undefined) {
    throw problemError(result.error, "The request could not be completed.");
  }
  return result.data;
}

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

export function useCurrentUser() {
  const navigate = useNavigate();
  return useQuery({
    queryKey: ["auth", "me"],
    queryFn: async (): Promise<User> => {
      const result = await getAuthMe();
      try {
        return unwrapData(result).user;
      } catch (error) {
        if (error instanceof SessionExpiredError) {
          void navigate("/login", { replace: true });
        }
        throw error;
      }
    },
    retry: (count, error) => !(error instanceof SessionExpiredError) && count < 1,
  });
}
