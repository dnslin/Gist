import { readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const STABLE_SEMVER = /^[0-9]+\.[0-9]+\.[0-9]+$/;

export interface VersionCheckOptions {
  root: string;
  tag?: string;
  goVersion?: string;
  viteVersion?: string;
  dockerVersion?: string;
}

function readText(filePath: string): string {
  try {
    return readFileSync(filePath, "utf8");
  } catch (error) {
    throw new Error(`cannot read ${filePath}: ${String(error)}`);
  }
}

function requireMatch(value: string, expected: string, label: string): void {
  if (value !== expected) {
    throw new Error(`${label} is ${JSON.stringify(value)}, expected ${JSON.stringify(expected)}`);
  }
}

function parseVersionFile(root: string): string {
  const raw = readText(path.join(root, "VERSION"));
  if (!/^[0-9]+\.[0-9]+\.[0-9]+(?:\r?\n)?$/.test(raw)) {
    throw new Error("VERSION must contain exactly one stable SemVer X.Y.Z line");
  }
  const version = raw.replace(/\r?\n$/, "");
  if (!STABLE_SEMVER.test(version)) {
    throw new Error("VERSION must be stable SemVer X.Y.Z without a v prefix");
  }
  return version;
}

export function checkProductVersion(options: VersionCheckOptions): string {
  const root = path.resolve(options.root);
  const version = parseVersionFile(root);

  const packageMetadata = JSON.parse(
    readText(path.join(root, "frontend", "package.json")),
  ) as { version?: unknown };
  requireMatch(String(packageMetadata.version ?? ""), version, "frontend package version");

  const goConfig = readText(path.join(root, "backend", "internal", "config", "config.go"));
  const goFallback = goConfig.match(/^var AppVersion = "([^"]+)"$/m)?.[1] ?? "";
  requireMatch(goFallback, version, "Go fallback version");

  const serverMain = readText(path.join(root, "backend", "cmd", "server", "main.go"));
  const annotation = serverMain.match(/^\/\/ @version (.+)$/m)?.[1] ?? "";
  requireMatch(annotation, version, "Swagger source version");

  const swaggerJson = JSON.parse(
    readText(path.join(root, "backend", "docs", "swagger.json")),
  ) as { info?: { version?: unknown } };
  requireMatch(String(swaggerJson.info?.version ?? ""), version, "Swagger JSON version");

  const swaggerYaml = readText(path.join(root, "backend", "docs", "swagger.yaml"));
  const yamlVersion = swaggerYaml.match(/^\s{2}version: (.+)$/m)?.[1] ?? "";
  requireMatch(yamlVersion, version, "Swagger YAML version");

  const swaggerGo = readText(path.join(root, "backend", "docs", "docs.go"));
  const goVersion = swaggerGo.match(/^\s*Version:\s+"([^"]+)"/m)?.[1] ?? "";
  requireMatch(goVersion, version, "Swagger Go version");

  if (options.tag !== undefined) requireMatch(options.tag, `v${version}`, "Git tag");
  if (options.goVersion !== undefined) requireMatch(options.goVersion, version, "Go injected version");
  if (options.viteVersion !== undefined) requireMatch(options.viteVersion, version, "Vite injected version");
  if (options.dockerVersion !== undefined) requireMatch(options.dockerVersion, version, "Docker image version");

  return version;
}

function parseArgs(args: string[]): VersionCheckOptions {
  let root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
  const options: VersionCheckOptions = { root };

  for (let index = 0; index < args.length; index += 1) {
    const flag = args[index];
    const value = args[index + 1];
    if (!value) throw new Error(`${flag} requires a value`);
    switch (flag) {
      case "--root":
        root = value;
        options.root = value;
        break;
      case "--tag":
        options.tag = value;
        break;
      case "--go-version":
        options.goVersion = value;
        break;
      case "--vite-version":
        options.viteVersion = value;
        break;
      case "--docker-version":
        options.dockerVersion = value;
        break;
      default:
        throw new Error(`unknown argument: ${flag}`);
    }
    index += 1;
  }
  return options;
}

const entryPath = process.argv[1] ? path.resolve(process.argv[1]) : "";
if (entryPath === fileURLToPath(import.meta.url)) {
  try {
    process.stdout.write(`${checkProductVersion(parseArgs(process.argv.slice(2)))}\n`);
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : String(error)}\n`);
    process.exitCode = 1;
  }
}
