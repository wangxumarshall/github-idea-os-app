import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({
    defaultLayout: undefined,
    onLayoutChanged: vi.fn(),
  }),
}));

vi.mock("@/shared/api", () => ({
  api: {},
}));

vi.mock("@/features/auth", () => ({
  useAuthStore: () => ({
    isLoading: false,
  }),
}));

vi.mock("@/features/workspace", () => ({
  useWorkspaceStore: () => ({
    skills: [],
    upsertSkill: vi.fn(),
    removeSkill: vi.fn(),
  }),
}));

vi.mock("@/hooks/use-mobile", () => ({
  useIsMobile: () => false,
}));

vi.mock("@/shared/hooks/use-query-param-selection", () => ({
  useQueryParamSelection: () => ["", vi.fn()],
}));

vi.mock("./file-tree", () => ({
  FileTree: () => null,
}));

vi.mock("./file-viewer", () => ({
  FileViewer: () => null,
}));

import { CreateSkillDialog, detectSkillImportSource } from "./skills-page";

describe("detectSkillImportSource", () => {
  it("recognizes GitHub URLs", () => {
    expect(
      detectSkillImportSource("https://github.com/acme/skills-repo/tree/main/skills/code-review"),
    ).toBe("github");
  });
});

describe("CreateSkillDialog", () => {
  it("shows GitHub loading copy for GitHub imports", async () => {
    const user = userEvent.setup();
    let resolveImport: (() => void) | undefined;
    const onImport = vi.fn(() => new Promise<void>((resolve) => {
      resolveImport = resolve;
    }));

    render(
      <CreateSkillDialog
        onClose={vi.fn()}
        onCreate={vi.fn()}
        onImport={onImport}
      />,
    );

    await user.click(screen.getByRole("tab", { name: "Import" }));
    await user.type(
      screen.getByPlaceholderText("Paste a skill URL..."),
      "https://github.com/acme/skills-repo/tree/main/skills/code-review",
    );

    expect(screen.getByText("GitHub")).toBeInTheDocument();
    expect(
      screen.getByText("GitHub accepts either a skill folder page or a `SKILL.md` file page."),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: /^Import$/ }));

    expect(onImport).toHaveBeenCalledWith(
      "https://github.com/acme/skills-repo/tree/main/skills/code-review",
    );
    expect(
      screen.getByRole("button", { name: "Importing from GitHub..." }),
    ).toBeInTheDocument();

    resolveImport?.();
  });
});
