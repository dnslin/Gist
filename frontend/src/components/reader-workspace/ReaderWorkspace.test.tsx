import type { ReactNode } from "react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import { Router } from "wouter";
import { memoryLocation } from "wouter/memory-location";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@/components/i18n-provider";
import { TooltipProvider } from "@/components/ui/tooltip";

const {
  mockUseMobileLayout,
  mockUseSelection,
  mockUseMobileDocumentScrollMode,
} = vi.hoisted(() => ({
  mockUseMobileLayout: vi.fn(),
  mockUseSelection: vi.fn(),
  mockUseMobileDocumentScrollMode: vi.fn(),
}));

vi.mock("react-i18next", async (importOriginal) => ({
  ...(await importOriginal<typeof import("react-i18next")>()),
  I18nextProvider: ({ children }: { children: ReactNode }) => children,
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/hooks/useMobileLayout", () => ({
  useMobileLayout: mockUseMobileLayout,
}));

vi.mock("@/hooks/useSelection", () => ({
  useSelection: mockUseSelection,
  selectionToParams: vi.fn(() => ({})),
}));

vi.mock("@/hooks/useEntries", () => ({
  useMarkAllAsRead: vi.fn(() => ({ mutate: vi.fn() })),
  useEntry: vi.fn(() => ({ data: undefined })),
}));

vi.mock("@/hooks/useFeeds", () => ({
  useFeeds: vi.fn(() => ({ data: [] })),
}));

vi.mock("@/hooks/useFolders", () => ({
  useFolders: vi.fn(() => ({ data: [] })),
}));

vi.mock("@/hooks/useAppearanceSettings", () => ({
  useAppearanceSettings: vi.fn(() => ({
    data: { contentTypes: ["article"] },
    isLoading: false,
  })),
}));

vi.mock("@/hooks/useRefreshStatus", () => ({
  useRefreshStatus: vi.fn(),
}));

vi.mock("@/hooks/useUISettings", () => ({
  useUISettingKey: vi.fn(() => true),
  useUISettingActions: vi.fn(() => ({ toggleSidebarVisible: vi.fn() })),
  hasSidebarVisibilitySetting: vi.fn(() => true),
  setUISetting: vi.fn(),
}));

vi.mock("@/hooks/useMobileDocumentScrollMode", () => ({
  useMobileDocumentScrollMode: mockUseMobileDocumentScrollMode,
}));

vi.mock("@/hooks/useTitle", () => ({
  buildTitle: vi.fn(() => "Gist"),
  useTitle: vi.fn(),
}));

vi.mock("@/components/layout/three-column-layout", () => ({
  ThreeColumnLayout: ({
    sidebar,
    list,
    content,
    hideList,
    showSidebar,
  }: {
    sidebar?: ReactNode;
    list?: ReactNode;
    content?: ReactNode;
    hideList?: boolean;
    showSidebar?: boolean;
  }) => (
    <main
      aria-label="reader workspace"
      data-hide-list={String(Boolean(hideList))}
      data-show-sidebar={String(showSidebar)}
    >
      <div data-slot="sidebar">{sidebar}</div>
      <div data-slot="list">{list}</div>
      <div data-slot="content">{content}</div>
    </main>
  ),
}));

vi.mock("@/components/ui/sheet", () => ({
  Sheet: ({
    open,
    children,
  }: {
    open: boolean;
    children: ReactNode;
  }) => (
    <section aria-label="mobile sidebar" data-open={String(open)}>
      {children}
    </section>
  ),
}));

vi.mock("@/components/sidebar", () => ({
  Sidebar: () => <nav aria-label="reader navigation" />,
}));

vi.mock("@/components/add-feed", () => ({
  AddFeedPage: () => <section aria-label="add feed" />,
}));

vi.mock("@/components/entry-list", () => ({
  EntryList: () => <section aria-label="entry list" />,
}));

vi.mock("@/components/entry-content", () => ({
  EntryContent: () => <article aria-label="entry content" />,
}));

vi.mock("@/components/picture-masonry", () => ({
  PictureMasonry: () => <section aria-label="picture masonry" />,
  Lightbox: () => <div data-testid="lightbox" />,
}));

vi.mock("@/components/layout/ScrollToTopZone", () => ({
  ScrollToTopZone: () => <div data-testid="scroll-to-top" />,
}));

vi.mock("@/components/ui/image-preview", () => ({
  ImagePreview: () => <div data-testid="image-preview" />,
}));

import { ReaderWorkspace } from "./ReaderWorkspace";

function selectionState(overrides: Record<string, unknown> = {}) {
  return {
    selection: { type: "all" as const },
    selectAll: vi.fn(),
    selectFeed: vi.fn(),
    selectFolder: vi.fn(),
    selectStarred: vi.fn(),
    selectedEntryId: null,
    selectEntry: vi.fn(),
    unreadOnly: false,
    toggleUnreadOnly: vi.fn(),
    contentType: "article" as const,
    ...overrides,
  };
}

function mobileLayoutState(overrides: Record<string, unknown> = {}) {
  return {
    isMobile: false,
    isTablet: false,
    mobileView: "list" as const,
    sidebarOpen: false,
    setSidebarOpen: vi.fn(),
    showList: vi.fn(),
    openSidebar: vi.fn(),
    closeSidebar: vi.fn(),
    ...overrides,
  };
}

function renderWorkspace(path: string) {
  const location = memoryLocation({ path });
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <I18nProvider>
        <TooltipProvider delayDuration={300}>
          <Router hook={location.hook} searchHook={location.searchHook}>
            <ReaderWorkspace />
          </Router>
        </TooltipProvider>
      </I18nProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  mockUseMobileLayout.mockReturnValue(mobileLayoutState());
  mockUseSelection.mockReturnValue(selectionState());
});

afterEach(() => {
  cleanup();
});

describe("ReaderWorkspace", () => {
  it("renders the existing desktop article workspace composition", () => {
    const { container } = renderWorkspace("/all?type=article");

    expect(
      screen.getByRole("main", { name: "reader workspace" }),
    ).toBeTruthy();
    expect(
      screen.getByRole("navigation", { name: "reader navigation" }),
    ).toBeTruthy();
    expect(screen.getByRole("region", { name: "entry list" })).toBeTruthy();
    expect(screen.getByText("entry.select_article")).toBeTruthy();
    expect(screen.getByTestId("image-preview")).toBeTruthy();
    expect(screen.queryByTestId("lightbox")).toBeNull();
    expect(container.children).toHaveLength(2);
    expect(mockUseMobileDocumentScrollMode).toHaveBeenCalledWith({
      enabled: false,
      locked: false,
    });
  });

  it("keeps the mobile list, detail, overlays, and scroll mode together", async () => {
    mockUseMobileLayout.mockReturnValue(
      mobileLayoutState({
        isMobile: true,
        mobileView: "detail",
        sidebarOpen: true,
      }),
    );
    mockUseSelection.mockReturnValue(
      selectionState({ selectedEntryId: "entry-1" }),
    );

    const { container } = renderWorkspace("/all/entry-1?type=article");

    const listPage = container.querySelector(".mobile-list-page");
    const detailPage = container.querySelector(".mobile-detail-page");
    const sheet = screen.getByRole("region", { name: "mobile sidebar" });

    expect(listPage?.getAttribute("aria-hidden")).toBe("true");
    expect(listPage?.hasAttribute("inert")).toBe(true);
    expect(detailPage?.getAttribute("aria-hidden")).toBe("false");
    expect(detailPage?.hasAttribute("inert")).toBe(false);
    expect(await screen.findByRole("article", { name: "entry content" })).toBeTruthy();
    expect(sheet.getAttribute("data-open")).toBe("true");
    expect(
      sheet.contains(
        screen.getByRole("navigation", { name: "reader navigation" }),
      ),
    ).toBe(true);
    expect(container.children[1]?.getAttribute("data-testid")).toBe(
      "scroll-to-top",
    );
    expect(container.children[2]?.getAttribute("data-testid")).toBe(
      "image-preview",
    );
    expect(container.children[3]).toBe(sheet);
    expect(mockUseMobileDocumentScrollMode).toHaveBeenCalledWith({
      enabled: true,
      locked: true,
    });
  });
});
