import { afterEach, describe, expect, test } from "bun:test";
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
import { checkProductVersion } from "./version";

const roots: string[] = [];

function createFixture(version = "1.2.0\n"): string {
  const root = mkdtempSync(path.join(tmpdir(), "gist-version-"));
  roots.push(root);
  mkdirSync(path.join(root, "frontend"), { recursive: true });
  mkdirSync(path.join(root, "backend", "cmd", "server"), { recursive: true });
  mkdirSync(path.join(root, "backend", "internal", "config"), { recursive: true });
  mkdirSync(path.join(root, "backend", "docs"), { recursive: true });
  writeFileSync(path.join(root, "VERSION"), version);
  writeFileSync(path.join(root, "frontend", "package.json"), '{"version":"1.2.0"}\n');
  writeFileSync(path.join(root, "backend", "internal", "config", "config.go"), 'var AppVersion = "1.2.0"\n');
  writeFileSync(path.join(root, "backend", "cmd", "server", "main.go"), "// @version 1.2.0\n");
  writeFileSync(path.join(root, "backend", "docs", "swagger.json"), '{"info":{"version":"1.2.0"}}\n');
  writeFileSync(path.join(root, "backend", "docs", "swagger.yaml"), "info:\n  version: 1.2.0\n");
  writeFileSync(path.join(root, "backend", "docs", "docs.go"), 'var SwaggerInfo = &swag.Spec{\n\tVersion:          "1.2.0",\n}\n');
  return root;
}

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

describe("checkProductVersion", () => {
  test("returns the single validated product version", () => {
    const root = createFixture();
    expect(checkProductVersion({ root, tag: "v1.2.0", goVersion: "1.2.0", viteVersion: "1.2.0", dockerVersion: "1.2.0" })).toBe("1.2.0");
  });

  test("accepts a Windows line ending", () => {
    const root = createFixture("1.2.0\r\n");
    expect(checkProductVersion({ root })).toBe("1.2.0");
  });

  test.each(["", "v1.2.0\n", "1.2\n", "1.2.0-beta\n", "1.2.0\n1.2.1\n"])(
    "rejects invalid VERSION content %p",
    (contents) => {
      const root = createFixture(contents);
      expect(() => checkProductVersion({ root })).toThrow("VERSION must contain exactly one stable SemVer");
    },
  );

  test("rejects missing VERSION", () => {
    const root = createFixture();
    rmSync(path.join(root, "VERSION"));
    expect(() => checkProductVersion({ root })).toThrow("cannot read");
  });

  test.each([
    ["frontend/package.json", '{"version":"1.2.1"}\n', "frontend package version"],
    ["backend/internal/config/config.go", 'var AppVersion = "1.2.1"\n', "Go fallback version"],
    ["backend/cmd/server/main.go", "// @version 1.2.1\n", "Swagger source version"],
    ["backend/docs/swagger.json", '{"info":{"version":"1.2.1"}}\n', "Swagger JSON version"],
    ["backend/docs/swagger.yaml", "info:\n  version: 1.2.1\n", "Swagger YAML version"],
    ["backend/docs/docs.go", 'var SwaggerInfo = &swag.Spec{\n\tVersion:          "1.2.1",\n}\n', "Swagger Go version"],
  ])("rejects metadata drift in %s", (relativePath, contents, label) => {
    const root = createFixture();
    writeFileSync(path.join(root, relativePath), contents);
    expect(() => checkProductVersion({ root })).toThrow(label);
  });

  test.each([
    [{ tag: "v1.2.1" }, "Git tag"],
    [{ goVersion: "1.2.1" }, "Go injected version"],
    [{ viteVersion: "1.2.1" }, "Vite injected version"],
    [{ dockerVersion: "1.2.1" }, "Docker image version"],
  ])("rejects injected consumer drift", (extra, label) => {
    const root = createFixture();
    expect(() => checkProductVersion({ root, ...extra })).toThrow(label);
  });
});
