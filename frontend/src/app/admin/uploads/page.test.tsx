import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
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

function installFetchMock(initial: unknown) {
  const fetchMock = vi.fn().mockImplementation((path: string) => {
    if (path === "/api/images") {
      return Promise.resolve(new Response(JSON.stringify(initial), { status: 200 }));
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
