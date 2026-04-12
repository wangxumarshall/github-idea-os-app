import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { AgentRuntime, User } from "@/shared/types";

const {
  mockCreateAdminSshSession,
  mockGetAdminSshSession,
  mockCloseAdminSshSession,
  mockFit,
  mockScrollToBottom,
  mockFocus,
  mockTerminalConstructor,
} = vi.hoisted(() => {
  const mockCreateAdminSshSession = vi.fn();
  const mockGetAdminSshSession = vi.fn();
  const mockCloseAdminSshSession = vi.fn();
  const mockFit = vi.fn();
  const mockScrollToBottom = vi.fn();
  const mockFocus = vi.fn();
  const mockTerminalConstructor = vi.fn();

  return {
    mockCreateAdminSshSession,
    mockGetAdminSshSession,
    mockCloseAdminSshSession,
    mockFit,
    mockScrollToBottom,
    mockFocus,
    mockTerminalConstructor,
  };
});

let currentUser: User | null = null;

vi.mock("@/shared/api", () => ({
  api: {
    createAdminSshSession: mockCreateAdminSshSession,
    getAdminSshSession: mockGetAdminSshSession,
    closeAdminSshSession: mockCloseAdminSshSession,
  },
}));

vi.mock("@/features/auth", () => ({
  useAuthStore: (selector: (state: { user: User | null }) => unknown) =>
    selector({ user: currentUser }),
}));

vi.mock("@xterm/xterm", () => ({
  Terminal: class MockTerminal {
    cols = 120;
    rows = 36;

    constructor(options: unknown) {
      mockTerminalConstructor(options);
    }

    loadAddon() {}
    open() {}
    focus() {
      mockFocus();
    }
    writeln() {}
    clear() {}
    write() {}
    scrollToBottom() {
      mockScrollToBottom();
    }
    dispose() {}
    onData() {
      return { dispose() {} };
    }
    onResize() {
      return { dispose() {} };
    }
  },
}));

vi.mock("@xterm/addon-fit", () => ({
  FitAddon: class MockFitAddon {
    fit() {
      mockFit();
    }
  },
}));

const runtime: AgentRuntime = {
  id: "runtime-1",
  workspace_id: "workspace-1",
  daemon_id: "daemon-1",
  name: "Codex (VM-0-3-ubuntu)",
  runtime_mode: "local",
  provider: "codex",
  status: "online",
  device_info: "VM-0-3-ubuntu",
  metadata: {
    ssh_enabled: true,
    ssh_host: "127.0.0.1",
    ssh_port: 22,
    ssh_user: "ubuntu",
    cli_version: "0.1.25",
  },
  last_seen_at: "2026-04-12T00:00:00Z",
  created_at: "2026-04-12T00:00:00Z",
  updated_at: "2026-04-12T00:00:00Z",
};

beforeEach(() => {
  vi.clearAllMocks();
  currentUser = {
    id: "user-1",
    name: "Admin",
    email: "admin@example.com",
    avatar_url: null,
    is_super_admin: true,
    created_at: "2026-04-12T00:00:00Z",
    updated_at: "2026-04-12T00:00:00Z",
  };
  mockGetAdminSshSession.mockResolvedValue(null);
  Object.defineProperty(globalThis.navigator, "clipboard", {
    configurable: true,
    value: {
      writeText: vi.fn(),
    },
  });
  localStorage.clear();
});

import { RuntimeSshTerminal } from "./runtime-ssh-terminal";

describe("RuntimeSshTerminal", () => {
  it("initializes xterm with extended scrollback and scroll controls", async () => {
    const user = userEvent.setup();
    render(<RuntimeSshTerminal runtime={runtime} />);

    await waitFor(() => {
      expect(screen.getByLabelText("Enter fullscreen terminal")).toBeEnabled();
    });

    expect(mockTerminalConstructor).toHaveBeenCalledWith(
      expect.objectContaining({ scrollback: 50000 }),
    );

    await user.click(screen.getByLabelText("Scroll terminal to bottom"));
    expect(mockScrollToBottom).toHaveBeenCalled();
  });

  it("opens and closes the fullscreen dialog", async () => {
    const user = userEvent.setup();
    render(<RuntimeSshTerminal runtime={runtime} />);

    await waitFor(() => {
      expect(screen.getByLabelText("Enter fullscreen terminal")).toBeEnabled();
    });

    await user.click(screen.getByLabelText("Enter fullscreen terminal"));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).getByText("SSH Terminal")).toBeInTheDocument();
    expect(within(dialog).queryByText(/Fullscreen session for/i)).not.toBeInTheDocument();

    await user.click(within(dialog).getByLabelText("Exit fullscreen terminal"));

    await waitFor(() => {
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    });
  });

  it("hides terminal metadata panels in fullscreen mode", async () => {
    const user = userEvent.setup();
    render(<RuntimeSshTerminal runtime={runtime} />);

    expect(screen.getByText("SSH target")).toBeInTheDocument();
    expect(screen.getByText("tmux mode")).toBeInTheDocument();
    expect(screen.getByText("Session")).toBeInTheDocument();

    await user.click(screen.getByLabelText("Enter fullscreen terminal"));

    const dialog = await screen.findByRole("dialog");
    expect(within(dialog).queryByText("SSH target")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("tmux mode")).not.toBeInTheDocument();
    expect(within(dialog).queryByText("Session")).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/Fullscreen session for/i)).not.toBeInTheDocument();
    expect(within(dialog).queryByText(/Host verification stays strict/i)).not.toBeInTheDocument();
  });
});
