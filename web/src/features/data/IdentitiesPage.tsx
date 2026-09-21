import { type FormEvent, useRef, useState } from "react";
import { useNavigate, useParams } from "react-router";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  archiveIdentity,
  createIdentity,
  getIdentity,
  listIdentities,
  listObservations,
  patchIdentity,
} from "@/api/generated/sdk.gen";
import type { Identity } from "@/api/generated/types.gen";
import { problemMessage, refreshCsrf } from "@/api/http";
import {
  modesForPlatform,
  platformLabel,
  PLATFORMS,
  type AccountMode,
} from "@/features/data/platforms";
import { newIdempotencyKey } from "@/shared/lib/id";
import { readEtag, unwrapOrRedirect, useCurrentUser } from "@/shared/lib/session";
import { ActionButton } from "@/shared/ui/ActionButton";
import { DataTable } from "@/shared/ui/DataTable";
import { Dialog } from "@/shared/ui/Dialog";
import { QueryRecovery } from "@/shared/ui/QueryRecovery";
import { Select, TextInput } from "@/shared/ui/FormControls";
import styles from "@/features/pages.module.css";

interface IdentityForm {
  platformId: string;
  kind: "main" | "subordinate";
  parentIdentityId: string;
  externalRef: string;
  displayName: string;
  accountMode: AccountMode | "";
  status: "enabled" | "disabled";
}

const emptyForm: IdentityForm = {
  platformId: PLATFORMS[0].id,
  kind: "main",
  parentIdentityId: "",
  externalRef: "",
  displayName: "",
  accountMode: PLATFORMS[0].modes[0],
  status: "enabled",
};

export function IdentitiesPage() {
  const { portfolioId = "" } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const user = useCurrentUser();
  const createKey = useRef(newIdempotencyKey());
  const [form, setForm] = useState<IdentityForm>(emptyForm);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [etag, setEtag] = useState("");
  const [frozen, setFrozen] = useState(false);
  const [archiveId, setArchiveId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const redirect = () => {
    void navigate("/login", { replace: true });
  };
  const identities = useQuery({
    queryKey: ["identities", user.data?.id, portfolioId],
    enabled: Boolean(user.data?.id && portfolioId),
    queryFn: async () =>
      unwrapOrRedirect(await listIdentities({ path: { portfolioId } }), redirect),
  });
  const mains = (identities.data?.items ?? []).filter(
    (item) => item.kind === "main" && item.status !== "archived",
  );
  const modes = modesForPlatform(form.platformId);

  function applyIdentity(item: Identity, nextEtag: string, nextFrozen: boolean) {
    setEditingId(item.id);
    setEtag(nextEtag);
    setFrozen(nextFrozen);
    setForm({
      platformId: item.platformId,
      kind: item.kind,
      parentIdentityId: item.parentIdentityId ?? "",
      externalRef: item.externalRef,
      displayName: item.displayName,
      accountMode: item.accountMode,
      status: item.status === "disabled" ? "disabled" : "enabled",
    });
  }

  async function startEdit(identityId: string) {
    setError(null);
    setNotice(null);
    const result = await getIdentity({ path: { portfolioId, identityId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    const item = unwrapOrRedirect(result, redirect);
    const observed = await listObservations({
      path: { portfolioId },
      query: { identityId, limit: 1 },
    });
    if (observed.response?.status === 401) {
      redirect();
      return;
    }
    applyIdentity(
      item,
      readEtag(result.response, `"identity:${String(item.version)}"`),
      (observed.data?.items.length ?? 0) > 0,
    );
  }

  async function loadLatest(identityId: string, message: string) {
    const result = await getIdentity({ path: { portfolioId, identityId } });
    if (result.response?.status === 401) {
      redirect();
      return;
    }
    if (result.data) {
      applyIdentity(
        result.data,
        readEtag(result.response, `"identity:${String(result.data.version)}"`),
        frozen,
      );
    }
    setError(message);
  }

  async function invalidate() {
    await queryClient.invalidateQueries({ queryKey: ["identities", user.data?.id, portfolioId] });
  }

  async function onSubmit(event: FormEvent) {
    event.preventDefault();
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      await refreshCsrf();
      if (editingId) {
        const result = await patchIdentity({
          path: { portfolioId, identityId: editingId },
          headers: { "If-Match": etag },
          body: {
            displayName: form.displayName,
            status: form.status,
            ...(frozen
              ? {}
              : {
                  externalRef: form.externalRef,
                  accountMode: form.accountMode || undefined,
                  parentIdentityId:
                    form.kind === "subordinate" ? form.parentIdentityId || null : null,
                }),
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.response?.status === 412) {
          await loadLatest(
            editingId,
            "The identity changed in another session. The form now shows the latest server values.",
          );
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The identity could not be updated."));
          return;
        }
        applyIdentity(
          result.data,
          readEtag(result.response, `"identity:${String(result.data.version)}"`),
          frozen,
        );
        setNotice("Identity updated.");
      } else {
        const result = await createIdentity({
          path: { portfolioId },
          headers: { "Idempotency-Key": createKey.current },
          body: {
            platformId: form.platformId,
            displayName: form.displayName,
            externalRef: form.externalRef,
            accountMode: form.accountMode || PLATFORMS[0].modes[0],
            kind: form.kind,
            parentIdentityId: form.kind === "subordinate" ? form.parentIdentityId : null,
          },
        });
        if (result.response?.status === 401) {
          redirect();
          return;
        }
        if (result.error || !result.data) {
          setError(problemMessage(result.error, "The identity could not be created."));
          return;
        }
        createKey.current = newIdempotencyKey();
        setForm({ ...emptyForm, platformId: form.platformId, accountMode: form.accountMode });
        setNotice("Identity created.");
      }
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be saved."));
    } finally {
      setBusy(false);
    }
  }

  async function confirmArchive() {
    if (!archiveId) {
      return;
    }
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const current = await getIdentity({ path: { portfolioId, identityId: archiveId } });
      if (current.response?.status === 401) {
        redirect();
        return;
      }
      const item = unwrapOrRedirect(current, redirect);
      const match = readEtag(current.response, `"identity:${String(item.version)}"`);
      await refreshCsrf();
      const result = await archiveIdentity({
        path: { portfolioId, identityId: archiveId },
        headers: { "If-Match": match },
      });
      if (result.response?.status === 401) {
        redirect();
        return;
      }
      if (result.response?.status === 412) {
        setArchiveId(null);
        await loadLatest(
          archiveId,
          "The identity changed in another session. Reload before archiving.",
        );
        return;
      }
      if (result.error) {
        setError(problemMessage(result.error, "The identity could not be archived."));
        return;
      }
      if (editingId === archiveId) {
        setEditingId(null);
        setForm(emptyForm);
        setEtag("");
        setFrozen(false);
      }
      setArchiveId(null);
      setNotice("Identity archived.");
      await invalidate();
    } catch (cause) {
      setError(problemMessage(cause, "The identity could not be archived."));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className={styles.wide}>
      <h2>Identities</h2>
      <p className={styles.lead}>
        Platform login subjects stay in this Portfolio. After the first Observation, identity fields
        other than name and status freeze.
      </p>
      {error ? (
        <p className={styles.alert} role="alert">
          {error}
        </p>
      ) : null}
      {notice ? <p className={styles.notice}>{notice}</p> : null}
      <QueryRecovery
        error={identities.error}
        hasData={Boolean(identities.data)}
        busy={identities.isFetching}
        fallback="Identities could not be loaded."
        onRetry={() => {
          void identities.refetch();
        }}
      />
      <form className={styles.form} onSubmit={(event) => void onSubmit(event)}>
        <label className={styles.field}>
          <span>Platform</span>
          <Select
            value={form.platformId}
            disabled={Boolean(editingId)}
            onChange={(event) => {
              const platformId = event.target.value;
              const nextModes = modesForPlatform(platformId);
              setForm((current) => ({
                ...current,
                platformId,
                accountMode: nextModes[0] ?? "",
              }));
            }}
            required
          >
            {PLATFORMS.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </label>
        <label className={styles.field}>
          <span>Kind</span>
          <Select
            value={form.kind}
            disabled={Boolean(editingId)}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                kind: event.target.value as IdentityForm["kind"],
                parentIdentityId: event.target.value === "main" ? "" : current.parentIdentityId,
              }))
            }
            required
          >
            <option value="main">Main</option>
            <option value="subordinate">Subordinate</option>
          </Select>
        </label>
        {form.kind === "subordinate" ? (
          <label className={styles.field}>
            <span>Parent identity</span>
            <Select
              value={form.parentIdentityId}
              disabled={Boolean(editingId) && frozen}
              onChange={(event) =>
                setForm((current) => ({ ...current, parentIdentityId: event.target.value }))
              }
              required
            >
              <option value="" disabled>
                Select a main identity
              </option>
              {mains.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.displayName}
                </option>
              ))}
            </Select>
          </label>
        ) : null}
        <label className={styles.field}>
          <span>Display name</span>
          <TextInput
            value={form.displayName}
            onChange={(event) =>
              setForm((current) => ({ ...current, displayName: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>External ref</span>
          <TextInput
            value={form.externalRef}
            disabled={Boolean(editingId) && frozen}
            onChange={(event) =>
              setForm((current) => ({ ...current, externalRef: event.target.value }))
            }
            required
          />
        </label>
        <label className={styles.field}>
          <span>Account mode</span>
          <Select
            value={form.accountMode}
            disabled={Boolean(editingId) && frozen}
            onChange={(event) =>
              setForm((current) => ({
                ...current,
                accountMode: event.target.value as AccountMode,
              }))
            }
            required
          >
            {modes.map((mode) => (
              <option key={mode} value={mode}>
                {mode}
              </option>
            ))}
          </Select>
        </label>
        {editingId ? (
          <label className={styles.field}>
            <span>Status</span>
            <Select
              value={form.status}
              onChange={(event) =>
                setForm((current) => ({
                  ...current,
                  status: event.target.value as IdentityForm["status"],
                }))
              }
            >
              <option value="enabled">Enabled</option>
              <option value="disabled">Disabled</option>
            </Select>
          </label>
        ) : null}
        {frozen ? (
          <p className={styles.muted}>
            Observations exist for this identity, so platform fields can no longer change.
          </p>
        ) : null}
        <div className={styles.actions}>
          <ActionButton type="submit" tone="primary" disabled={busy}>
            {editingId ? "Save identity" : "Create identity"}
          </ActionButton>
          {editingId ? (
            <ActionButton
              type="button"
              onClick={() => {
                setEditingId(null);
                setForm(emptyForm);
                setEtag("");
                setFrozen(false);
              }}
            >
              Cancel edit
            </ActionButton>
          ) : null}
        </div>
      </form>
      <DataTable
        caption="Portfolio identities"
        columns={[
          { key: "name", header: "Name" },
          { key: "platform", header: "Platform" },
          { key: "kind", header: "Kind" },
          { key: "mode", header: "Account mode" },
          { key: "status", header: "Status" },
          { key: "externalRef", header: "External ref" },
        ]}
        rows={(identities.data?.items ?? []).map((item) => ({
          id: item.id,
          name: item.displayName,
          platform: platformLabel(item.platformId),
          kind: item.kind,
          mode: item.accountMode,
          status: item.status,
          externalRef: item.externalRef,
        }))}
      />
      <ul className={styles.flowActions}>
        {(identities.data?.items ?? []).map((item) => (
          <li key={item.id}>
            <span>{item.displayName}</span>
            <span className={styles.actions}>
              <ActionButton type="button" disabled={busy} onClick={() => void startEdit(item.id)}>
                Edit
              </ActionButton>
              {item.status !== "archived" ? (
                <ActionButton
                  type="button"
                  tone="dangerQuiet"
                  disabled={busy}
                  onClick={() => setArchiveId(item.id)}
                >
                  Archive
                </ActionButton>
              ) : null}
            </span>
          </li>
        ))}
      </ul>
      <Dialog open={Boolean(archiveId)} title="Archive identity" onClose={() => setArchiveId(null)}>
        <p className={styles.lead}>
          Archived identities stay attached to this Portfolio and cannot be reused as live
          collection subjects.
        </p>
        <div className={styles.actions}>
          <ActionButton
            type="button"
            tone="danger"
            disabled={busy}
            onClick={() => void confirmArchive()}
          >
            Archive
          </ActionButton>
          <ActionButton type="button" onClick={() => setArchiveId(null)}>
            Cancel
          </ActionButton>
        </div>
      </Dialog>
    </section>
  );
}
