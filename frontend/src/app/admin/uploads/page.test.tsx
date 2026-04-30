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

  it("shows a toast after Copy URL is clicked", async () => {
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

    await waitFor(() => {
      expect(screen.getByText(/url copied/i)).toBeDefined();
    });
  });
});

describe("UploadsPage — delete flow", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("deletes an unused image without confirmation", async () => {
    let deleteCalls = 0;
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      if (path === "/api/images/1700000001_b.png" && init?.method === "DELETE") {
        deleteCalls++;
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) return Promise.resolve(new Response(JSON.stringify({ filename: m[1], pages: [] }), { status: 200 }));
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000001_b.png"));

    fireEvent.click(screen.getByText("1700000001_b.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    await waitFor(() => {
      expect(deleteCalls).toBe(1);
      expect(screen.queryByText("1700000001_b.png")).toBeNull();
    });
  });

  it("opens confirm modal for a referenced image and force-deletes on confirm", async () => {
    const calls: string[] = [];
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      calls.push(`${init?.method ?? "GET"} ${path}`);
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) {
        const fname = decodeURIComponent(m[1]);
        const pages = fname === "1700000000_a.png" ? ["blog/post", "about"] : [];
        return Promise.resolve(new Response(JSON.stringify({ filename: fname, pages }), { status: 200 }));
      }
      if (path === "/api/images/1700000000_a.png?force=1" && init?.method === "DELETE") {
        return Promise.resolve(new Response(null, { status: 204 }));
      }
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));
    fireEvent.click(screen.getByText("1700000000_a.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    // Confirm modal lists the pages.
    expect(await screen.findByTestId("confirm-modal")).toBeDefined();
    expect(screen.getByText("/blog/post")).toBeDefined();
    expect(screen.getByText("/about")).toBeDefined();

    fireEvent.click(screen.getByTestId("confirm-delete"));

    await waitFor(() => {
      expect(calls).toContain("DELETE /api/images/1700000000_a.png?force=1");
      expect(screen.queryByText("1700000000_a.png")).toBeNull();
    });
  });

  it("opens confirm modal when DELETE returns 409 for an item the grid showed as unused", async () => {
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) {
        const fname = decodeURIComponent(m[1]);
        return Promise.resolve(new Response(JSON.stringify({ filename: fname, pages: [] }), { status: 200 }));
      }
      if (path === "/api/images/1700000001_b.png" && init?.method === "DELETE") {
        return Promise.resolve(
          new Response(JSON.stringify({ error: "in use", pages: ["unexpected/page"] }), { status: 409 }),
        );
      }
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000001_b.png"));
    fireEvent.click(screen.getByText("1700000001_b.png").parentElement!);
    fireEvent.click(await screen.findByTestId("delete-button"));

    expect(await screen.findByTestId("confirm-modal")).toBeDefined();
    expect(screen.getByText("/unexpected/page")).toBeDefined();
  });
});

describe("UploadsPage — upload", () => {
  beforeEach(() => {
    localStorage.setItem("access_token", "tok");
  });

  it("shows a toast when upload returns 400 with an error body", async () => {
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      if (path === "/api/images" && init?.method === "POST") {
        return Promise.resolve(
          new Response(
            JSON.stringify({ error: "invalid file type, only images allowed" }),
            { status: 400 },
          ),
        );
      }
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    const dropzone = screen.getByTestId("dropzone");
    const file = new File(["hello"], "bad.txt", { type: "text/plain" });
    const dataTransfer = { files: [file], items: [], types: ["Files"] };
    fireEvent.drop(dropzone, { dataTransfer });

    await waitFor(() => {
      expect(screen.getByText(/invalid file type, only images allowed/i)).toBeDefined();
    });
  });

  it("dropping a file POSTs to /api/images and prepends the response", async () => {
    const newInfo = {
      filename: "1700000999_c.png",
      url: "/uploads/1700000999_c.png",
      size: 100,
      ref_count: 0,
      uploaded_at: "2023-11-14T22:14:59Z",
    };
    const fetchMock = vi.fn().mockImplementation((path: string, init?: RequestInit) => {
      if (path === "/api/images" && (!init || init.method === undefined)) {
        return Promise.resolve(new Response(JSON.stringify(sample), { status: 200 }));
      }
      if (path === "/api/images" && init?.method === "POST") {
        return Promise.resolve(new Response(JSON.stringify(newInfo), { status: 201 }));
      }
      const m = path.match(/^\/api\/images\/(.+)\/refs$/);
      if (m) return Promise.resolve(new Response(JSON.stringify({ filename: m[1], pages: [] }), { status: 200 }));
      return Promise.resolve(new Response("", { status: 404 }));
    });
    vi.stubGlobal("fetch", fetchMock);

    render(<UploadsPage />);
    await waitFor(() => screen.getByText("1700000000_a.png"));

    const dropzone = screen.getByTestId("dropzone");
    const file = new File(["hello"], "c.png", { type: "image/png" });
    const dataTransfer = { files: [file], items: [], types: ["Files"] };
    fireEvent.drop(dropzone, { dataTransfer });

    await waitFor(() => {
      expect(screen.getByText("1700000999_c.png")).toBeDefined();
    });
  });
});
