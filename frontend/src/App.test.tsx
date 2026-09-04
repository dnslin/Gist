import type { ReactNode } from "react";
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const { mockUseAuth } = vi.hoisted(() => ({
  mockUseAuth: vi.fn(),
}));

vi.mock("react-i18next", () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}));

vi.mock("@/hooks/useAuth", () => ({
  useAuth: mockUseAuth,
}));

vi.mock("@/components/reader-workspace", () => ({
  ReaderWorkspace: () => <main aria-label="reader workspace" />,
}));

vi.mock("@/components/auth", () => ({
  LoginPage: () => <main aria-label="login" />,
  RegisterPage: () => <main aria-label="registration" />,
  NetworkErrorPage: () => <main aria-label="network error" />,
}));

vi.mock("@/components/update-notice", () => ({
  UpdateNotice: () => <aside aria-label="update notice" />,
}));

vi.mock("@/components/ui/tooltip", () => ({
  TooltipProvider: ({ children }: { children: ReactNode }) => children,
}));

import App from "./App";

function authState(overrides: Record<string, unknown> = {}) {
  return {
    isLoading: false,
    isAuthenticated: false,
    needsRegistration: false,
    needsLogin: false,
    isNetworkError: false,
    error: null,
    shouldRedirectToRoot: false,
    login: vi.fn(),
    register: vi.fn(),
    retry: vi.fn(),
    clearError: vi.fn(),
    consumeRootRedirect: vi.fn(),
    ...overrides,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  window.history.replaceState(null, "", "/all?type=article");
  mockUseAuth.mockReturnValue(authState());
});

afterEach(() => {
  cleanup();
});

describe("App", () => {
  it("renders the shared reader workspace after authentication", () => {
    mockUseAuth.mockReturnValue(authState({ isAuthenticated: true }));

    const { container } = render(<App />);

    expect(
      screen.getByRole("main", { name: "reader workspace" }),
    ).toBeTruthy();
    expect(screen.getByRole("complementary", { name: "update notice" })).toBeTruthy();
    expect(container.firstElementChild?.classList.contains("app-shell")).toBe(
      true,
    );
  });

  it("keeps the loading state in the Web shell", () => {
    mockUseAuth.mockReturnValue(authState({ isLoading: true }));

    render(<App />);

    expect(screen.getByText("entry.loading")).toBeTruthy();
    expect(
      screen.queryByRole("main", { name: "reader workspace" }),
    ).toBeNull();
  });

  it("keeps the network error state in the Web shell", () => {
    mockUseAuth.mockReturnValue(authState({ isNetworkError: true }));

    render(<App />);

    expect(screen.getByRole("main", { name: "network error" })).toBeTruthy();
  });

  it("keeps the registration state in the Web shell", () => {
    mockUseAuth.mockReturnValue(authState({ needsRegistration: true }));

    render(<App />);

    expect(screen.getByRole("main", { name: "registration" })).toBeTruthy();
  });

  it("keeps the login state in the Web shell", () => {
    mockUseAuth.mockReturnValue(authState({ needsLogin: true }));

    render(<App />);

    expect(screen.getByRole("main", { name: "login" })).toBeTruthy();
  });
});
