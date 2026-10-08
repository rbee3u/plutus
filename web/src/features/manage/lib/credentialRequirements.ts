import type { CredentialRequirement } from "@/api";

/** Browser requirements preserve the catalog's explicit selection policy. */
export function credentialRequired(
  requirement: CredentialRequirement,
  selected: string[],
): boolean {
  switch (requirement.mode) {
    case "always":
      return true;
    case "selected":
      return selected.some((type) => requirement.accountTypes.includes(type));
    case "selected_except":
      return selected.some((type) => !requirement.accountTypes.includes(type));
  }
}
