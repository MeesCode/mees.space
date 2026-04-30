import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { fireEvent } from "@testing-library/react";
import UploadsPage from "./page";

const sample = [
  {
    filename: "1700000000_a.png",
    url: "/uploads/1700000000_a.png",
    size: 1024,
    ref_count: 2,
    uploaded_at: "2023-11-14T22:13:20Z",
  },
  {
    filename: "1700000001_b.png",
    url: "/uploads/1700000001_b.png",
    size: 2048,
    ref_count: 0,
    uploaded_at: "2023-11-14T22:13:21Z",
  },
];

function installFetchMock(initial: unknown, refsByFile: Record<string, string[]> = {}) {
  const fetchMock = vi.fn().mockImplementation((path: string) => {
    if (path === "/api/images") {
      return Promise.resolve(new Response(JSON.stringify(initial), { status: 200 }));
    }
    const m = path.match(/^\/api\/images\/(.+)\/refs$/);
    if (m) {
      const filename = decodeURIComponent(m[1]);
      const pages = refsByFile[filename] ?? [];
      return Promise.resolve(new Response(JSON.stringify({ filename, pages }), { status: 200 }));
    }
    return Promise.resolve(new Response("", { status: 404 }));
  });
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

describe("UploadsPage — grid skeleton", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("renders one thumbnail per ImageInfo with ref_count badge", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByText("1700000000_a.png")).toBeDefined();
      expect(screen.getByText("1700000001_b.png")).toBeDefined();
    });

    expect(screen.getByTestId("ref-badge-1700000000_a.png").textContent).toBe("2");
    expect(screen.getByTestId("ref-badge-1700000001_b.png").textContent).toBe("0");
  });

  it("shows the totals header", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("totals").textContent).toContain("2 images");
      expect(screen.getByTestId("totals").textContent).toContain("1 unused");
    });
  });

  it("renders ? badge when ref_count is -1", async () => {
    installFetchMock([{ ...sample[0], ref_count: -1 }]);
    render(<UploadsPage />);

    await waitFor(() => {
      expect(screen.getByTestId("ref-badge-1700000000_a.png").textContent).toBe("?");
    });
  });
});

describe("UploadsPage — filters, sort, detail rail", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("'unused only' hides referenced items", async () => {
    installFetchMock(sample);
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByTestId("filter-unused"));

    expect(screen.queryByText("1700000000_a.png")).toBeNull();
    expect(screen.getByText("1700000001_b.png")).toBeDefined();
  });

  it("sorting by size puts the biggest first", async () => {
    installFetchMock(sample); // a=1024, b=2048
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByTestId("sort-size"));

    const tiles = screen.getAllByTestId(/^ref-badge-/);
    expect(tiles[0].getAttribute("data-testid")).toBe("ref-badge-1700000001_b.png");
  });

  it("clicking a thumb populates the detail rail", async () => {
    installFetchMock(sample, { "1700000000_a.png": ["blog/post"] });
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);

    await waitFor(() => {
      expect(screen.getByTestId("detail-filename").textContent).toBe("1700000000_a.png");
      expect(screen.getByTestId("detail-size").textContent).toContain("1.0 KB");
    });
  });

  it("Copy URL writes the public URL to the clipboard", async () => {
    installFetchMock(sample);
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", {
      value: { writeText },
      configurable: true,
    });
    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);
    fireEvent.click(screen.getByTestId("copy-url"));

    expect(writeText).toHaveBeenCalledWith("/uploads/1700000000_a.png");
  });
});
