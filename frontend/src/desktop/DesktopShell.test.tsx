import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import type { ReactNode } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

const desktopMainSource = readFileSync(
  resolve(process.cwd(), "src/desktop/main.tsx"),
  "utf8",
);
const desktopShellSource = readFileSync(
  resolve(process.cwd(), "src/desktop/DesktopShell.tsx"),
  "utf8",
);
const desktopHtmlSource = readFileSync(
  resolve(process.cwd(), "desktop/index.html"),
  "utf8",
);
const desktopEntrySource = `${desktopMainSource}\n${desktopShellSource}`;
const desktopEntryImports = [desktopMainSource, desktopShellSource].flatMap(
  (source) =>
    Array.from(
      source.matchAll(
        /^import(?:\s+.*\s+from)?\s+["']([^"']+)["'];?$/gm,
      ),
      (match) => match[1]!,
    ),
);
const originalServiceWorker = Object.getOwnPropertyDescriptor(
  navigator,
  "serviceWorker",
);

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  vi.doUnmock("react-dom/client");
  vi.resetModules();
  document.body.replaceChildren();

  if (originalServiceWorker) {
    Object.defineProperty(
      navigator,
      "serviceWorker",
      originalServiceWorker,
    );
  } else {
    Reflect.deleteProperty(navigator, "serviceWorker");
  }
});

describe("DesktopShell", () => {
  it("renders the desktop resource probe independently", async () => {
    const { DesktopShell } = await import("./DesktopShell");

    render(<DesktopShell />);

    const shell = screen.getByRole("main", { name: "Gist desktop shell" });
    expect(shell.textContent).toBe("Gist");
  });

  it("boots without API, PWA, or update-check side effects", async () => {
    const rootElement = document.createElement("div");
    rootElement.id = "root";
    document.body.replaceChildren(rootElement);

    const fetch = vi.fn();
    const serviceWorker = {
      register: vi.fn(),
      getRegistration: vi.fn(),
      getRegistrations: vi.fn(),
    };
    const cacheStorage = {
      keys: vi.fn(),
      delete: vi.fn(),
    };

    vi.stubGlobal("fetch", fetch);
    vi.stubGlobal("caches", cacheStorage);
    Object.defineProperty(navigator, "serviceWorker", {
      configurable: true,
      value: serviceWorker,
    });
    const rootRender = vi.fn((node: ReactNode) => {
      render(node, { container: rootElement });
    });
    const createRoot = vi.fn(() => ({ render: rootRender }));
    vi.doMock("react-dom/client", () => ({ createRoot }));

    await import("./main");

    expect(createRoot).toHaveBeenCalledWith(rootElement);
    expect(rootRender).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("main", { name: "Gist desktop shell" }).textContent,
    ).toBe("Gist");
    expect(fetch).not.toHaveBeenCalled();
    expect(serviceWorker.register).not.toHaveBeenCalled();
    expect(serviceWorker.getRegistration).not.toHaveBeenCalled();
    expect(serviceWorker.getRegistrations).not.toHaveBeenCalled();
    expect(cacheStorage.keys).not.toHaveBeenCalled();
    expect(cacheStorage.delete).not.toHaveBeenCalled();
  });

  it("keeps the desktop entry isolated from the product and PWA entries", () => {
    expect(desktopEntryImports).toEqual([
      "react",
      "react-dom/client",
      "../index.css",
      "./DesktopShell",
    ]);
    expect(
      desktopEntryImports.filter((specifier) => specifier.endsWith(".css")),
    ).toEqual(["../index.css"]);
    expect(desktopHtmlSource).toContain(
      '<script type="module" src="../src/desktop/main.tsx"></script>',
    );

    expect(desktopEntrySource).not.toMatch(/\bApp\b/);
    expect(desktopEntrySource).not.toMatch(/UpdateNotice|update-notice/);
    expect(desktopEntrySource).not.toMatch(/serviceWorker|virtual:pwa-register/);
    expect(desktopEntrySource).not.toMatch(/@\/api|["'`]\/api\b|\bfetch\s*\(/);
    expect(desktopHtmlSource).not.toMatch(
      /manifest\.webmanifest|serviceWorker|boot-guard|sw-preload-fix/,
    );
    expect(desktopShellSource).not.toMatch(/\bstyle\s*=/);
    expect(desktopHtmlSource).not.toMatch(
      /<style\b|<link\b[^>]*\brel=["']stylesheet["']/i,
    );
  });
});
