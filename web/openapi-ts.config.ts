import { defineConfig } from "@hey-api/openapi-ts";

export default defineConfig({
  input: "../api/openapi/openapi.yaml",
  output: "src/api/generated",
});
