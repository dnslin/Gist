import { existsSync, readdirSync, readFileSync, statSync } from "node:fs";
import {
  basename,
  dirname,
  extname,
  join,
  relative,
  resolve,
  sep,
} from "node:path";
import { fileURLToPath } from "node:url";

const scriptPath = fileURLToPath(import.meta.url);
const scriptDirectory = dirname(scriptPath);
const defaultOutputDirectory = resolve(
  scriptDirectory,
  "../../desktop/frontend/dist",
);

const pwaPublicAssets = new Set([
  "apple-touch-icon-180x180.png",
  "maskable-icon-512x512.png",
  "pwa-64x64.png",
  "pwa-192x192.png",
  "pwa-512x512.png",
  "sw-preload-fix.js",
]);

const forbiddenPaths = [
  { label: "Web App entry", pattern: /^assets\/app-[^/]+\.js$/i },
  {
    label: "PWA manifest",
    pattern: /(?:^|\/)(?:[^/]*\.webmanifest|manifest\.json)$/i,
  },
  {
    label: "Service Worker",
    pattern:
      /(?:^|\/)(?:sw|service-worker|serviceworker|register-?sw)(?:[.-][^/]*)?\.[cm]?js(?:\.map)?$/i,
  },
  { label: "Workbox", pattern: /(?:^|\/)[^/]*workbox[^/]*$/i },
];

const forbiddenContents = [
  {
    label: "PWA manifest link",
    pattern: /<link\b[^>]*\brel=["']manifest["']|manifest\.webmanifest/i,
  },
  { label: "Service Worker code", pattern: /\bserviceWorker\b/ },
  { label: "Workbox code", pattern: /\bworkbox\b/i },
  {
    label: "Web update prompt",
    pattern:
      /UpdateNotice|update-notice|update\.available|update\.description|virtual:pwa-register/,
  },
  {
    label: "Web App boot code",
    pattern: /data-gist-boot-ready|\[boot-guard\]/,
  },
];

const textExtensions = new Set([
  ".cjs",
  ".css",
  ".html",
  ".js",
  ".json",
  ".map",
  ".mjs",
  ".svg",
  ".txt",
  ".webmanifest",
]);

function listFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name);
    return entry.isDirectory() ? listFiles(path) : [path];
  });
}

export function verifyDesktopAssets(outputDirectory: string): number {
  const entryPath = join(outputDirectory, "index.html");
  const toOutputPath = (path: string) =>
    relative(outputDirectory, path).split(sep).join("/");

  if (!existsSync(entryPath) || !statSync(entryPath).isFile()) {
    throw new Error(
      `Desktop asset verification failed: missing ${toOutputPath(entryPath)}`,
    );
  }

  const files = listFiles(outputDirectory);
  const violations: string[] = [];

  for (const path of files) {
    const outputPath = toOutputPath(path);

    if (pwaPublicAssets.has(basename(outputPath).toLowerCase())) {
      violations.push(`PWA public asset: ${outputPath}`);
    }

    for (const forbidden of forbiddenPaths) {
      if (forbidden.pattern.test(outputPath)) {
        violations.push(`${forbidden.label}: ${outputPath}`);
      }
    }

    if (!textExtensions.has(extname(path).toLowerCase())) continue;

    const content = readFileSync(path, "utf8");

    for (const forbidden of forbiddenContents) {
      if (forbidden.pattern.test(content)) {
        violations.push(`${forbidden.label}: ${outputPath}`);
      }
    }
  }

  if (violations.length > 0) {
    const details = [...new Set(violations)]
      .sort()
      .map((violation) => `- ${violation}`)
      .join("\n");
    throw new Error(`Desktop asset verification failed:\n${details}`);
  }

  return files.length;
}

if (process.argv[1] && resolve(process.argv[1]) === scriptPath) {
  try {
    const fileCount = verifyDesktopAssets(defaultOutputDirectory);
    console.info(`Desktop assets verified: ${fileCount} files`);
  } catch (error) {
    console.error(error instanceof Error ? error.message : error);
    process.exitCode = 1;
  }
}
