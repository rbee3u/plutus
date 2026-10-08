import tseslint from "typescript-eslint";
import { fileURLToPath, URL } from "node:url";
import { Linter } from "eslint";
import { describe, expect, it } from "vitest";
import boundaries from "./eslint.boundaries.js";

function messages(file, code) {
  const filename = fileURLToPath(new URL(`./src/${file}`, import.meta.url));
  return new Linter().verify(
    code,
    [
      {
        files: ["**/*.ts", "**/*.tsx"],
        languageOptions: { parser: tseslint.parser },
        plugins: { architecture: { rules: { boundaries } } },
        rules: { "architecture/boundaries": "error" },
      },
    ],
    { filename },
  );
}

describe("architectural imports", () => {
  it.each([
    ["features/manage/Example.tsx", 'import { x } from "@/api/generated/sdk.gen";'],
    ["shared/lib/example.ts", 'export * from "../../api/client";'],
    ["app/App.tsx", 'import("../api/keys");'],
    ["features/manage/Example.tsx", 'type X = import("../../api/generated/types.gen").Platform;'],
    ["shared/lib/example.ts", 'type X = import("@/app/shell/navigation").NavItem;'],
    ["shared/lib/example.ts", 'import { x } from "@/features/holdings/display";'],
    ["shared/lib/example.ts", 'export * from "../../app/App";'],
    ["features/holdings/pages/Example.tsx", 'import { x } from "../../../app/shell/Shell";'],
    [
      "features/overview/Example.tsx",
      'import { x } from "@/features/holdings/components/HoldingsTree";',
    ],
    ["features/overview/Example.tsx", 'export { x } from "../holdings/components/HoldingsTree";'],
    [
      "features/overview/Example.tsx",
      'import("../holdings/./components/../components/HoldingsTree");',
    ],
    ["api/example.ts", 'import { x } from "../shared/lib/session";'],
  ])("rejects forbidden dependencies from %s: %s", (file, code) => {
    expect(messages(file, code)).toEqual([
      expect.objectContaining({ ruleId: "architecture/boundaries" }),
    ]);
  });

  it.each([
    ["features/overview/Example.tsx", 'import { x } from "@/features/holdings/display";'],
    ["features/overview/Example.tsx", 'export { x } from "../history/timeWindow.ts";'],
    ["features/holdings/pages/Example.tsx", 'import { x } from "../components/HoldingsTree";'],
    ["features/overview/Example.tsx", 'import { x } from "@/shared/ui/DataTable";'],
    ["app/App.tsx", 'import { x } from "@/features/holdings/pages/BalancesPage";'],
    ["shared/lib/example.ts", 'import { x } from "@/api";'],
    ["shared/lib/example.ts", 'export * from "../../api/index.ts";'],
    ["api/index.ts", 'export * from "./generated/sdk.gen";'],
    ["features/new-feature/Example.tsx", 'import { x } from "react";'],
  ])("allows intended dependencies from %s: %s", (file, code) => {
    expect(messages(file, code)).toEqual([]);
  });
});
