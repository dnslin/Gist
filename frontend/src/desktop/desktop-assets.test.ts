import {
  mkdirSync,
  mkdtempSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import { verifyDesktopAssets } from "../../scripts/verify-desktop-assets";

const temporaryDirectories: string[] = [];

function createOutput(files: Record<string, string> = {}): string {
  const outputDirectory = mkdtempSync(
    join(tmpdir(), "gist-desktop-assets-"),
  );
  temporaryDirectories.push(outputDirectory);

  const outputFiles = {
    "index.html": '<main aria-label="Gist desktop shell">Gist</main>',
    "assets/desktop.js": 'console.info("desktop")',
    "assets/desktop.css": "body { margin: 0; }",
    ...files,
  };

  for (const [outputPath, content] of Object.entries(outputFiles)) {
    const path = join(outputDirectory, outputPath);
    mkdirSync(dirname(path), { recursive: true });
    writeFileSync(path, content);
  }

  return outputDirectory;
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("desktop asset verification", () => {
  it("accepts desktop assets, including shared logo and locales", () => {
    const outputDirectory = createOutput({
      "logo.svg": "<svg></svg>",
      "locales/en-US/translation.json": '{"title":"Gist"}',
    });

    expect(verifyDesktopAssets(outputDirectory)).toBe(5);
  });

  it("rejects output without an HTML entry", () => {
    const outputDirectory = mkdtempSync(
      join(tmpdir(), "gist-desktop-assets-"),
    );
    temporaryDirectories.push(outputDirectory);

    expect(() => verifyDesktopAssets(outputDirectory)).toThrow(
      "missing index.html",
    );
  });

  it.each([
    ["Web App entry", "assets/app-deadbeef.js", "console.info('web')"],
    ["PWA manifest", "manifest.webmanifest", "{}"],
    ["PWA manifest", "manifest.json", "{}"],
    ["Service Worker", "sw.js", "self.addEventListener('fetch', () => {})"],
    ["Workbox", "assets/workbox-deadbeef.js", "console.info('workbox')"],
    [
      "Web update prompt",
      "assets/desktop.js",
      "const updateKey = 'update.available'",
    ],
  ])("rejects %s output", (label, outputPath, content) => {
    const outputDirectory = createOutput({ [outputPath]: content });

    expect(() => verifyDesktopAssets(outputDirectory)).toThrow(label);
  });

  it.each([
    "apple-touch-icon-180x180.png",
    "maskable-icon-512x512.png",
    "pwa-64x64.png",
    "pwa-192x192.png",
    "pwa-512x512.png",
    "sw-preload-fix.js",
    "nested/PWA-64X64.PNG",
  ])("rejects PWA public asset %s", (assetName) => {
    const outputDirectory = createOutput({ [assetName]: "PWA asset" });

    expect(() => verifyDesktopAssets(outputDirectory)).toThrow(
      `PWA public asset: ${assetName}`,
    );
  });
});
