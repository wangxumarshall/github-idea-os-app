import { forwardRef, useEffect, useState, type ReactNode } from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { IdeaDocument } from "@/shared/types";

vi.mock("next/navigation", () => ({
  useParams: () => ({ slug: "repo-brain" }),
}));

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
    ...props
  }: {
    children: ReactNode;
    href: string;
    [key: string]: any;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("@/features/editor", () => ({
  ContentEditor: forwardRef(function MockContentEditor(
    {
      defaultValue = "",
      onUpdate,
    }: {
      defaultValue?: string;
      onUpdate?: (markdown: string) => void;
    },
    _ref,
  ) {
    const [value, setValue] = useState(defaultValue);

    useEffect(() => {
      setValue(defaultValue);
    }, [defaultValue]);

    return (
      <textarea
        data-testid="idea-editor"
        value={value}
        onChange={(event) => {
          setValue(event.target.value);
          onUpdate?.(event.target.value);
        }}
      />
    );
  }),
}));

const mockGetIdea = vi.hoisted(() => vi.fn());
const mockUpdateIdea = vi.hoisted(() => vi.fn());
const mockRetryIdeaRepo = vi.hoisted(() => vi.fn());
const mockToastError = vi.hoisted(() => vi.fn());
const mockToastSuccess = vi.hoisted(() => vi.fn());

vi.mock("@/shared/api", () => ({
  api: {
    getIdea: (...args: any[]) => mockGetIdea(...args),
    updateIdea: (...args: any[]) => mockUpdateIdea(...args),
    retryIdeaRepo: (...args: any[]) => mockRetryIdeaRepo(...args),
  },
}));

vi.mock("sonner", () => ({
  toast: {
    error: (...args: any[]) => mockToastError(...args),
    success: (...args: any[]) => mockToastSuccess(...args),
  },
}));

import IdeaPage from "./page";

const initialIdea: IdeaDocument = {
  code: "idea0001",
  slug: "repo-brain",
  path: "ideas/repo-brain/repo-brain.md",
  title: "Repo Brain",
  summary: "Track repository knowledge.",
  tags: ["github"],
  project_repo_name: "repo-brain",
  project_repo_url: "",
  project_repo_status: "creating",
  created_at: "2026-04-01",
  updated_at: "2026-04-08",
  content: "# Notes\n\nInitial content.",
  sha: "sha-1",
};

function sleep(ms: number) {
  return new Promise((resolve) => {
    setTimeout(resolve, ms);
  });
}

describe("Idea editor autosave", () => {
  beforeEach(() => {
    mockGetIdea.mockReset();
    mockUpdateIdea.mockReset();
    mockRetryIdeaRepo.mockReset();
    mockToastError.mockReset();
    mockToastSuccess.mockReset();
    mockRetryIdeaRepo.mockResolvedValue({ message: "ok" });
    mockGetIdea.mockResolvedValue(initialIdea);
  });

  it("uses the latest synced sha for queued autosaves", async () => {
    let resolveFirstSave: ((value: IdeaDocument) => void) | null = null;

    mockUpdateIdea
      .mockImplementationOnce(
        () =>
          new Promise<IdeaDocument>((resolve) => {
            resolveFirstSave = resolve;
          }),
      )
      .mockResolvedValueOnce({
        ...initialIdea,
        sha: "sha-2",
        updated_at: "2026-04-09",
        content: "# Notes\n\nSecond draft.",
      });

    render(<IdeaPage />);

    const editor = await screen.findByTestId("idea-editor");

    fireEvent.change(editor, { target: { value: "# Notes\n\nFirst draft." } });
    await act(async () => {
      await sleep(3100);
    });

    expect(mockUpdateIdea).toHaveBeenCalledTimes(1);
    expect(mockUpdateIdea.mock.calls[0]?.[1]).toMatchObject({
      sha: "sha-1",
      base_content: "# Notes\n\nInitial content.",
    });

    fireEvent.change(editor, { target: { value: "# Notes\n\nSecond draft." } });
    await act(async () => {
      await sleep(3100);
    });

    expect(mockUpdateIdea).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveFirstSave?.({
        ...initialIdea,
        sha: "sha-2",
        updated_at: "2026-04-09",
        content: "# Notes\n\nFirst draft.",
      });
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(mockUpdateIdea).toHaveBeenCalledTimes(2);
    expect(mockUpdateIdea.mock.calls[1]?.[1]).toMatchObject({
      sha: "sha-2",
      base_content: "# Notes\n\nFirst draft.",
      content: "# Notes\n\nSecond draft.",
    });
  }, 15000);
});
