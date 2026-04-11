import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mockPush = vi.hoisted(() => vi.fn());
const mockReplace = vi.hoisted(() => vi.fn());
let currentSearch = "";

vi.mock("next/navigation", () => ({
  usePathname: () => "/agents",
  useRouter: () => ({
    push: (...args: any[]) => mockPush(...args),
    replace: (...args: any[]) => mockReplace(...args),
  }),
  useSearchParams: () => new URLSearchParams(currentSearch),
}));

import {
  buildQueryParamHref,
  useQueryParamSelection,
} from "./use-query-param-selection";

function TestHarness() {
  const [selected, setSelected] = useQueryParamSelection("agent");

  return (
    <div>
      <div data-testid="selected">{selected || "none"}</div>
      <button type="button" onClick={() => setSelected("agent-1")}>
        select
      </button>
      <button
        type="button"
        onClick={() => setSelected("", { replace: true })}
      >
        clear
      </button>
    </div>
  );
}

describe("useQueryParamSelection", () => {
  beforeEach(() => {
    currentSearch = "";
    mockPush.mockReset();
    mockReplace.mockReset();
  });

  it("builds hrefs while preserving other query params", () => {
    expect(buildQueryParamHref("/agents", "show=archived", "agent", "agent-1")).toBe(
      "/agents?show=archived&agent=agent-1",
    );
    expect(buildQueryParamHref("/agents", "show=archived&agent=agent-1", "agent", "")).toBe(
      "/agents?show=archived",
    );
  });

  it("reads the current selection from the query string", () => {
    currentSearch = "agent=agent-7";

    render(<TestHarness />);

    expect(screen.getByTestId("selected")).toHaveTextContent("agent-7");
  });

  it("pushes and replaces URLs through the router", async () => {
    currentSearch = "show=archived";
    const user = userEvent.setup();

    render(<TestHarness />);

    await user.click(screen.getByRole("button", { name: "select" }));
    expect(mockPush).toHaveBeenCalledWith(
      "/agents?show=archived&agent=agent-1",
      { scroll: false },
    );

    await user.click(screen.getByRole("button", { name: "clear" }));
    expect(mockReplace).toHaveBeenCalledWith(
      "/agents?show=archived",
      { scroll: false },
    );
  });
});
