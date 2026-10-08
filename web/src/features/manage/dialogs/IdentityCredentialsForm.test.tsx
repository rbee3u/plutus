import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import type { CredentialForm as FormDescription, CredentialGroup, Identity, Platform } from "@/api";
import { IdentityCredentialsForm } from "./IdentityCredentialsForm";

vi.mock("@/shared/lib/useAuthenticatedWrite", () => ({ useAuthenticatedWrite: () => vi.fn() }));

const primary: CredentialGroup = {
  keyLabel: "API key",
  secretLabel: "API secret",
  secretInput: "password",
  requirement: { mode: "always", accountTypes: [] },
};
const item: Identity = {
  id: "identity",
  portfolioId: "portfolio",
  platform: "new-platform",
  displayName: "Identity",
  accountStructure: "structure",
  status: "enabled",
  version: 1,
  connection: { status: "unconfigured", credentialsConfigured: false },
  accounts: [],
  createdAt: "2026-10-08T00:00:00Z",
  updatedAt: "2026-10-08T00:00:00Z",
};
function render(form: FormDescription, selected: string[] = []) {
  const platform: Platform = {
    id: "new-platform",
    name: "New Platform",
    adapterAvailable: true,
    accountStructures: [],
    credentialForm: form,
  };
  return renderToStaticMarkup(
    <IdentityCredentialsForm
      portfolioId="portfolio"
      item={{
        ...item,
        accounts: selected.map((type) => ({
          id: type,
          identityId: item.id,
          accountType: type,
          displayName: type,
          selected: true,
          version: 1,
        })),
      }}
      platform={platform}
      onClose={() => {}}
      onSaved={async () => {}}
    />,
  );
}
function field(html: string, label: string) {
  return [...html.matchAll(/<label\b[^>]*>([\s\S]*?)<\/label>/g)].find((match) =>
    match[1].includes(label),
  )?.[1];
}

describe("credential descriptor rendering", () => {
  it("renders a PEM-capable primary input without recognizing a Platform name", () => {
    const html = render({
      primary: {
        ...primary,
        secretLabel: "API secret or Ed25519 private key",
        secretInput: "multiline",
      },
      passphrase: "hidden",
      help: "PEM instructions",
    });
    expect(field(html, "Ed25519")).toContain("<textarea");
    expect(html).toContain("PEM instructions");
    expect(field(html, "Passphrase")).toBeUndefined();
  });
  it.each(["optional", "required"] as const)("preserves the %s passphrase form", (passphrase) => {
    const html = render({ primary, passphrase, help: "" });
    expect(field(html, "Passphrase")).toBeDefined();
    expect(field(html, "Passphrase")?.includes('required=""')).toBe(passphrase === "required");
  });
  it.each([
    [[], false, false],
    [["earn"], true, false],
    [["earn", "manual"], true, true],
    [["main"], true, true],
    [["coin_m"], false, true],
  ] as const)("preserves split credential requirements for %j", (selected, spot, futures) => {
    const html = render(
      {
        primary: {
          ...primary,
          keyLabel: "Spot API key",
          requirement: { mode: "selected", accountTypes: ["main", "earn"] },
        },
        futures: {
          ...primary,
          keyLabel: "Futures API key",
          requirement: { mode: "selected_except", accountTypes: ["earn"] },
        },
        passphrase: "hidden",
        help: "",
      },
      [...selected],
    );
    expect(field(html, "Spot API key")?.includes('required=""')).toBe(spot);
    expect(field(html, "Futures API key")?.includes('required=""')).toBe(futures);
  });
});
