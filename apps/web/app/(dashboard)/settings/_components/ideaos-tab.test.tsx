import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockGetIdeaOSConfig = vi.hoisted(() => vi.fn());
const mockUpdateIdeaOSConfig = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());

const authState = {
  user: { id: "user-1" },
};

const workspaceState = {
  workspace: { id: "ws-1", name: "Test Workspace" },
  members: [{ user_id: "user-1", role: "owner" }],
  agents: [
    {
      id: "agent-1",
      name: "Local Codex",
      description: "Default local coding agent",
      archived_at: null,
    },
  ],
};

vi.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: () => null,
  }),
}));

vi.mock("@/features/auth", () => ({
  useAuthStore: (selector?: any) => (selector ? selector(authState) : authState),
}));

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: Object.assign(
    (selector?: any) => (selector ? selector(workspaceState) : workspaceState),
    { getState: () => workspaceState },
  ),
}));

vi.mock("@/shared/api", () => ({
  api: {
    getIdeaOSConfig: (...args: any[]) => mockGetIdeaOSConfig(...args),
    updateIdeaOSConfig: (...args: any[]) => mockUpdateIdeaOSConfig(...args),
    startGitHubOAuth: vi.fn(),
    disconnectGitHubAccount: vi.fn(),
  },
}));

vi.mock("sonner", () => ({
  toast: {
    success: (...args: any[]) => mockToastSuccess(...args),
    error: (...args: any[]) => mockToastError(...args),
  },
}));

import { IdeaOSTab } from "./ideaos-tab";

describe("IdeaOSTab", () => {
  beforeEach(() => {
    mockGetIdeaOSConfig.mockReset();
    mockUpdateIdeaOSConfig.mockReset();
    mockToastSuccess.mockReset();
    mockToastError.mockReset();

    mockGetIdeaOSConfig.mockResolvedValue({
      repo_url: "https://github.com/test-owner/ideas",
      branch: "main",
      directory: "ideas",
      repo_visibility: "private",
      default_agent_ids: [],
      github_connected: true,
      github_login: "test-owner",
    });
  });

  it("saves the checked default idea agent", async () => {
    mockUpdateIdeaOSConfig.mockResolvedValue({
      repo_url: "https://github.com/test-owner/ideas",
      branch: "main",
      directory: "ideas",
      repo_visibility: "private",
      default_agent_ids: ["agent-1"],
      github_connected: true,
      github_login: "test-owner",
    });

    const user = userEvent.setup();

    render(<IdeaOSTab />);

    await screen.findByText("Local Codex");

    const checkbox = document.querySelector('[data-slot="checkbox"]');
    if (!(checkbox instanceof HTMLElement)) {
      throw new Error("expected checkbox to be rendered");
    }

    await user.click(checkbox);

    await waitFor(() => expect(checkbox).toHaveAttribute("data-checked"));

    await user.click(screen.getByRole("button", { name: "Save" }));

    await waitFor(() => {
      expect(mockUpdateIdeaOSConfig).toHaveBeenCalledWith(expect.objectContaining({
        default_agent_ids: ["agent-1"],
      }));
    });
    await waitFor(() => expect(checkbox).toHaveAttribute("data-checked"));
  });
});
