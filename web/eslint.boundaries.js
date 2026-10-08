import path from "node:path";
import { fileURLToPath, URL } from "node:url";

const sourceRoot = fileURLToPath(new URL("./src/", import.meta.url));
const featureExports = new Set([
  "holdings/display",
  "holdings/navigation",
  "history/timeWindow",
  "manage/navigation",
]);

function sourcePath(filename) {
  return path.relative(sourceRoot, filename).split(path.sep).join("/");
}

/** Resolve aliases and relative imports before checking ownership boundaries. */
export default {
  meta: {
    type: "problem",
    schema: [],
    messages: { boundary: "{{reason}}" },
  },
  create(context) {
    const filename = context.filename;
    const [area, feature] = sourcePath(filename).split("/");
    function check(node) {
      const specifier = node.source?.value;
      if (typeof specifier !== "string") return;
      let target;
      if (specifier.startsWith("@/")) {
        target = path.resolve(sourceRoot, specifier.slice(2));
      } else if (specifier.startsWith(".")) {
        target = path.resolve(path.dirname(filename), specifier);
      } else {
        return;
      }
      const relative = sourcePath(target).replace(/\.(?:ts|tsx|js|jsx)$/, "");
      const [targetArea, targetFeature] = relative.split("/");
      let reason;
      if (area !== "api" && targetArea === "api" && !["api", "api/index"].includes(relative)) {
        reason = "API consumers must use the public API entrypoint.";
      } else if (area === "api" && ["app", "features", "shared"].includes(targetArea)) {
        reason = "API mechanics cannot depend on application or presentation code.";
      } else if (area === "shared" && ["app", "features"].includes(targetArea)) {
        reason = "Shared code cannot depend on the app or a feature.";
      } else if (area === "features" && targetArea === "app") {
        reason = "Features cannot depend on app assembly or the shell.";
      } else if (
        area === "features" &&
        targetArea === "features" &&
        feature !== targetFeature &&
        !featureExports.has(relative.slice("features/".length))
      ) {
        reason = "Cross-feature imports must use an explicit public feature entrypoint.";
      }
      if (reason) context.report({ node, messageId: "boundary", data: { reason } });
    }
    return {
      ImportDeclaration: check,
      ExportNamedDeclaration: check,
      ExportAllDeclaration: check,
      ImportExpression: check,
      TSImportType: check,
    };
  },
};
