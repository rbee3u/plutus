import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  archiveIdentity,
  disableIdentityConnection,
  enableIdentityConnection,
  keys,
  problemMessage,
  refreshCsrf,
  validateIdentityConnection,
} from "@/api";
import type { Identity, ValidationOutcome } from "@/api";
import { identityEtag } from "@/shared/lib/useIdentities";
import { useSessionScope } from "@/shared/lib/session";

const CONFLICT = "The identity changed in another session. The list was reloaded; retry.";

function isOutcome(value: unknown): value is ValidationOutcome {
  return (
    typeof value === "object" &&
    value !== null &&
    "ok" in value &&
    "identity" in value &&
    "validatedAt" in value
  );
}

function sentence(text: string): string {
  return /[.!?]$/.test(text) ? text : `${text}.`;
}

function outcomeText(outcome: ValidationOutcome): string {
  if (outcome.ok) {
    return `ok${outcome.externalRef ? ` (account ${outcome.externalRef})` : ""}`;
  }
  return `${outcome.errorClass ?? "failed"}${outcome.errorDetail ? `: ${outcome.errorDetail}` : ""}`;
}

/** Connection actions of one Identity row: validate, enable, disable, and archive. */
export function useIdentityActions(portfolioId: string) {
  const { userId, redirect } = useSessionScope();
  const queryClient = useQueryClient();
  const [archiveTarget, setArchiveTarget] = useState<Identity | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busyId, setBusyId] = useState<string | null>(null);

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: keys.identities(userId, portfolioId) });
  }

  function clearMessages() {
    setError(null);
    setNotice(null);
  }

  function notify(message: string) {
    setNotice(message);
  }

  async function runAction(
    item: Identity,
    action: () => Promise<{ status: number | undefined; message: string | null; error?: unknown }>,
    fallback: string,
  ) {
    setBusyId(item.id);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      const outcome = await action();
      if (outcome.status === 401) {
        redirect();
        return;
      }
      if (outcome.status === 412) {
        setError(CONFLICT);
        await invalidate();
        return;
      }
      if (outcome.message === null) {
        setError(problemMessage(outcome.error, fallback));
        return;
      }
      setNotice(outcome.message);
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, fallback));
    } finally {
      setBusyId(null);
    }
  }

  function validate(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await validateIdentityConnection({
          path: { portfolioId, identityId: item.id },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Validation of ${item.displayName}: ${sentence(outcomeText(result.data))} Connection is ${result.data.identity.connection.status}.`,
        };
      },
      "Validation could not run.",
    );
  }

  function enable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await enableIdentityConnection({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (result.response?.status === 422 && isOutcome(result.error)) {
          await invalidate();
          return {
            status: 422,
            message: null,
            error: new Error(
              `Enable failed for ${item.displayName}: ${sentence(outcomeText(result.error))} Connection is now ${result.error.identity.connection.status}.`,
            ),
          };
        }
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection enabled for ${item.displayName}: ${sentence(outcomeText(result.data))} It joins the next Collection Cycle.`,
        };
      },
      "The connection could not be enabled.",
    );
  }

  function disable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await disableIdentityConnection({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection disabled for ${item.displayName}. Its accounts leave the boundary at the next Collection Cycle.`,
        };
      },
      "The connection could not be disabled.",
    );
  }

  async function confirmArchive() {
    const item = archiveTarget;
    if (!item) {
      return;
    }
    await runAction(
      item,
      async () => {
        const result = await archiveIdentity({
          path: { portfolioId, identityId: item.id },
          headers: { "If-Match": identityEtag(item) },
        });
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        setArchiveTarget(null);
        return {
          status: result.response?.status,
          message: `Identity ${item.displayName} archived.`,
        };
      },
      "The identity could not be archived.",
    );
  }

  return {
    archiveTarget,
    setArchiveTarget,
    busyId,
    error,
    notice,
    clearMessages,
    notify,
    invalidate,
    validate,
    enable,
    disable,
    confirmArchive,
  };
}
