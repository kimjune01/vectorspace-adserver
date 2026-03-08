import { defineConfig } from "tsup";

export default defineConfig({
  entry: {
    index: "src/index.ts",
    viewability: "src/viewability.ts",
  },
  format: ["esm", "cjs"],
  dts: true,
  clean: true,
  sourcemap: true,
});
