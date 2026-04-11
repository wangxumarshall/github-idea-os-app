import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

let currentSearch = "";
let currentTabValue = "";

vi.mock("next/navigation", () => ({
  usePathname: () => "/settings",
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
  }),
  useSearchParams: () => new URLSearchParams(currentSearch),
}));

vi.mock("@/hooks/use-mobile", () => ({
  useIsMobile: () => true,
}));

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: (selector?: any) => {
    const state = {
      workspace: { id: "ws-1", name: "Mobile Workspace" },
    };
    return selector ? selector(state) : state;
  },
}));

vi.mock("@/components/ui/tabs", () => ({
  Tabs: ({ children, value }: any) => {
    currentTabValue = value ?? "";
    return <div>{children}</div>;
  },
  TabsList: ({ children }: any) => <div>{children}</div>,
  TabsTrigger: ({ children }: any) => <button type="button">{children}</button>,
  TabsContent: ({ children, value }: any) =>
    value === currentTabValue ? <div>{children}</div> : null,
}));

vi.mock("./_components/account-tab", () => ({
  AccountTab: () => <div>Account Content</div>,
}));

vi.mock("./_components/general-tab", () => ({
  AppearanceTab: () => <div>Appearance Content</div>,
}));

vi.mock("./_components/tokens-tab", () => ({
  TokensTab: () => <div>Tokens Content</div>,
}));

vi.mock("./_components/workspace-tab", () => ({
  WorkspaceTab: () => <div>Workspace Content</div>,
}));

vi.mock("./_components/members-tab", () => ({
  MembersTab: () => <div>Members Content</div>,
}));

vi.mock("./_components/repositories-tab", () => ({
  RepositoriesTab: () => <div>Repositories Content</div>,
}));

vi.mock("./_components/ideaos-tab", () => ({
  IdeaOSTab: () => <div>Ideas Content</div>,
}));

import SettingsPage from "./page";

describe("SettingsPage mobile layout", () => {
  beforeEach(() => {
    currentSearch = "";
    currentTabValue = "";
  });

  it("shows a settings section list when no tab is selected", () => {
    currentSearch = "";

    render(<SettingsPage />);

    expect(screen.getByText("Settings")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Profile/i })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Repositories/i })).toBeInTheDocument();
    expect(screen.queryByText("Account Content")).not.toBeInTheDocument();
  });

  it("shows the selected tab detail view on mobile", () => {
    currentSearch = "tab=profile";

    render(<SettingsPage />);

    expect(screen.getByRole("button", { name: /Settings/i })).toBeInTheDocument();
    expect(screen.getByText("Account Content")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Repositories/i })).not.toBeInTheDocument();
  });
});
