import { useAuthenticatedWrite } from "@/shared/lib/useAuthenticatedWrite";
import { useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import {
  archiveIdentity,
  disableIdentityConnection,
  enableIdentityConnection,
  keys,
  problemMessage,
  validateIdentityConnection,
} from "@/api";
import type { Identity, ValidationOutcome } from "@/api";
import { useSessionScope } from "@/shared/lib/session";

const CONFLICT = "The identity changed in another session. The list was reloaded; retry.";

function sentence(text: string): string {
  return /[.!?]$/.test(text) ? text : `${text}.`;
}

function outcomeText(outcome: ValidationOutcome): string {
  if (outcome.ok) {
    return `ok${outcome.externalRef ? ` (UID ${outcome.externalRef})` : ""}`;
  }
  return `${outcome.errorClass ?? "failed"}${outcome.errorDetail ? `: ${outcome.errorDetail}` : ""}`;
}

/** Connection actions of one identity row: validate, enable, disable, and archive. */
export function useIdentityActions(portfolioId: string) {
  const { userId } = useSessionScope();
  const write = useAuthenticatedWrite();
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

  async function runAction(
    item: Identity,
    action: () => Promise<{ status: number | undefined; message: string | null; error?: unknown }>,
    fallback: string,
  ) {
    setBusyId(item.id);
    clearMessages();
    try {
      const outcome = await action();
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
        const result = await write(() =>
          validateIdentityConnection({
            path: { portfolioId, identityId: item.id },
          }),
        );
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Validation of ${item.displayName}: ${sentence(outcomeText(result.data))} The connection is ${result.data.identity.connection.status}.`,
        };
      },
      "Validation could not run.",
    );
  }

  function enable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await write(() =>
          enableIdentityConnection({
            path: { portfolioId, identityId: item.id },
            headers: { "If-Edit-Version": String(item.version) },
          }),
        );
        if (result.response?.status === 422 && result.error?.validationOutcome) {
          await invalidate();
          return {
            status: 422,
            message: null,
            error: new Error(
              `Enable failed for ${item.displayName}: ${sentence(outcomeText(result.error.validationOutcome))} The connection is now ${result.error.validationOutcome.identity.connection.status}.`,
            ),
          };
        }
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection enabled for ${item.displayName}: ${sentence(outcomeText(result.data))} It joins the next collection cycle.`,
        };
      },
      "The connection could not be enabled.",
    );
  }

  function disable(item: Identity) {
    return runAction(
      item,
      async () => {
        const result = await write(() =>
          disableIdentityConnection({
            path: { portfolioId, identityId: item.id },
            headers: { "If-Edit-Version": String(item.version) },
          }),
        );
        if (!result.data) {
          return { status: result.response?.status, message: null, error: result.error };
        }
        return {
          status: result.response?.status,
          message: `Connection disabled for ${item.displayName}. Its accounts leave the boundary at the next collection cycle.`,
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
        const result = await write(() =>
          archiveIdentity({
            path: { portfolioId, identityId: item.id },
            headers: { "If-Edit-Version": String(item.version) },
          }),
        );
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
    notify: setNotice,
    invalidate,
    validate,
    enable,
    disable,
    confirmArchive,
  };
}
