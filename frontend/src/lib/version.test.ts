import { describe, expect, test } from "vitest";
import { readFileSync } from "node:fs";
import path from "node:path";
import { productVersion } from "@/lib/version";

describe("productVersion", () => {
  test("matches the validated repository version", () => {
    const expected = readFileSync(path.resolve(process.cwd(), "..", "VERSION"), "utf8").trim();
    expect(productVersion).toBe(expected);
  });
});
