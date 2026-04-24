import { describe, it, expect, vi, beforeEach } from "vitest";
import { apiFetch } from "./api";

function installFetchMock() {
  const fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
  return fetchMock;
}

function installLocationShim() {
  const loc = { href: "" };
  Object.defineProperty(window, "location", {
    value: loc,
    writable: true,
    configurable: true,
  });
  return loc;
}

describe("apiFetch", () => {
  let fetchMock: ReturnType<typeof installFetchMock>;
  let location: ReturnType<typeof installLocationShim>;

  beforeEach(() => {
    fetchMock = installFetchMock();
    location = installLocationShim();
  });

  it("sends no Authorization header when no token is set", async () => {
    fetchMock.mockResolvedValueOnce(new Response("ok", { status: 200 }));

    await apiFetch("/api/x");

    const headers = new Headers((fetchMock.mock.calls[0][1] as RequestInit).headers);
    expect(headers.has("Authorization")).toBe(false);
  });

  it("adds Authorization: Bearer <token> when access_token is set", async () => {
    localStorage.setItem("access_token", "tok123");
    fetchMock.mockResolvedValueOnce(new Response("ok", { status: 200 }));

    await apiFetch("/api/x");

    const headers = new Headers((fetchMock.mock.calls[0][1] as RequestInit).headers);
    expect(headers.get("Authorization")).toBe("Bearer tok123");
  });

  it("preserves caller-supplied headers alongside Authorization", async () => {
    localStorage.setItem("access_token", "tok");
    fetchMock.mockResolvedValueOnce(new Response("ok", { status: 200 }));

    await apiFetch("/api/x", { headers: { "Content-Type": "application/json" } });

    const headers = new Headers((fetchMock.mock.calls[0][1] as RequestInit).headers);
    expect(headers.get("Content-Type")).toBe("application/json");
    expect(headers.get("Authorization")).toBe("Bearer tok");
  });

  it("returns non-401 responses unchanged and does not attempt refresh", async () => {
    localStorage.setItem("access_token", "tok");
    const res = new Response("forbidden", { status: 403 });
    fetchMock.mockResolvedValueOnce(res);

    const result = await apiFetch("/api/x");

    expect(result).toBe(res);
  });

  it("on 401 with no refresh_token, clears tokens and redirects to /admin/login", async () => {
    localStorage.setItem("access_token", "tok");
    // no refresh_token set
    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));

    await apiFetch("/api/x");

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(location.href).toBe("/admin/login");
  });

  it("on 401 with refresh that succeeds, stores new tokens and retries with new Authorization", async () => {
    localStorage.setItem("access_token", "old-access");
    localStorage.setItem("refresh_token", "r1");

    // Initial request returns 401
    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));
    // Refresh call returns new tokens
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ access_token: "new-access", refresh_token: "r2" }),
        { status: 200 }
      )
    );
    // Retry returns 200
    const retryRes = new Response("ok", { status: 200 });
    fetchMock.mockResolvedValueOnce(retryRes);

    const result = await apiFetch("/api/x");

    expect(result).toBe(retryRes);
    expect(localStorage.getItem("access_token")).toBe("new-access");
    expect(localStorage.getItem("refresh_token")).toBe("r2");
    expect(fetchMock).toHaveBeenCalledTimes(3);

    const retryHeaders = new Headers((fetchMock.mock.calls[2][1] as RequestInit).headers);
    expect(retryHeaders.get("Authorization")).toBe("Bearer new-access");
  });

  it("on 401 with refresh that returns non-OK, clears tokens and redirects", async () => {
    localStorage.setItem("access_token", "tok");
    localStorage.setItem("refresh_token", "r1");

    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));
    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));

    await apiFetch("/api/x");

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(location.href).toBe("/admin/login");
  });

  it("on 401 with refresh that throws a network error, clears tokens and redirects", async () => {
    localStorage.setItem("access_token", "tok");
    localStorage.setItem("refresh_token", "r1");

    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));
    fetchMock.mockRejectedValueOnce(new Error("network down"));

    await apiFetch("/api/x");

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(location.href).toBe("/admin/login");
  });
});
