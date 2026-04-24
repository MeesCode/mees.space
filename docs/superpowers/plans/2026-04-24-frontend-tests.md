# Frontend Tests Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Vitest + JSDOM test harness to the Next.js frontend and cover the three files in `src/lib/` (`api.ts`, `auth.ts`, `navigation.tsx`).

**Architecture:** Vitest runs in Node with the `jsdom` environment for DOM APIs (localStorage, window.location, DOM events). Tests live next to their subjects as `*.test.ts` / `*.test.tsx`. React hook tests use `@testing-library/react`'s `renderHook`. No component rendering, no E2E, no real browser.

**Tech Stack:** Vitest, jsdom, @testing-library/react, @vitejs/plugin-react, TypeScript.

**Spec:** `docs/superpowers/specs/2026-04-24-frontend-tests-design.md`

---

## File Structure

**Create:**
- `frontend/vitest.config.ts` — Vitest config with jsdom + alias.
- `frontend/vitest.setup.ts` — global afterEach cleanup.
- `frontend/src/lib/api.test.ts` — `apiFetch()` tests.
- `frontend/src/lib/auth.test.ts` — `isLoggedIn()` / `logout()` tests.
- `frontend/src/lib/navigation.test.tsx` — `useNavigation()` tests.

**Modify:**
- `frontend/package.json` — add `test`, `test:watch` scripts; add devDependencies.

All work happens inside `frontend/`. Run every command from that directory unless noted.

---

## Task 1: Bootstrap Vitest tooling

**Files:**
- Modify: `frontend/package.json`
- Create: `frontend/vitest.config.ts`
- Create: `frontend/vitest.setup.ts`

Install the toolchain, add scripts, and verify `npm test` runs successfully with zero tests.

- [ ] **Step 1: Install devDependencies**

Run (from `frontend/`):

```bash
npm install --save-dev vitest jsdom @testing-library/react @vitejs/plugin-react
```

Expected: install completes without errors. `package.json` and `package-lock.json` are updated.

- [ ] **Step 2: Add scripts to package.json**

Edit `frontend/package.json`. In the `scripts` block, add two new entries so it reads:

```json
"scripts": {
  "dev": "next dev",
  "build": "next build",
  "lint": "next lint",
  "test": "vitest run",
  "test:watch": "vitest"
}
```

- [ ] **Step 3: Create `frontend/vitest.config.ts`**

```ts
import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    setupFiles: ["./vitest.setup.ts"],
    globals: false,
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
});
```

- [ ] **Step 4: Create `frontend/vitest.setup.ts`**

```ts
import { afterEach, vi } from "vitest";

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});
```

- [ ] **Step 5: Verify the test runner starts cleanly**

Run (from `frontend/`):

```bash
npm test -- --passWithNoTests
```

Expected: exit code 0. Output mentions "No test files found" or similar, and finishes without errors.

- [ ] **Step 6: Commit**

```bash
git add frontend/package.json frontend/package-lock.json frontend/vitest.config.ts frontend/vitest.setup.ts
git commit -m "test(frontend): bootstrap vitest + jsdom harness"
```

---

## Task 2: `api.test.ts` — token injection and 401 refresh flow

**Files:**
- Create: `frontend/src/lib/api.test.ts`

The `apiFetch` function (in `src/lib/api.ts`) handles: (a) adding a `Bearer` header when an access token is present, (b) on 401 attempting a refresh, retrying once on success, or clearing tokens and redirecting to `/admin/login` on failure. Eight tests cover these branches. Production code is already written — these are characterization tests.

- [ ] **Step 1: Create the test file**

Create `frontend/src/lib/api.test.ts`:

```ts
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
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("on 401 with no refresh_token, clears tokens and redirects to /admin/login", async () => {
    localStorage.setItem("access_token", "tok");
    // no refresh_token set
    fetchMock.mockResolvedValueOnce(new Response("", { status: 401 }));

    await apiFetch("/api/x");

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(location.href).toBe("/admin/login");
    expect(fetchMock).toHaveBeenCalledTimes(1);
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
    expect(fetchMock).toHaveBeenCalledTimes(2);
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
```

- [ ] **Step 2: Run the tests**

Run (from `frontend/`):

```bash
npm test -- src/lib/api.test.ts
```

Expected: 8 tests, all PASS, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/api.test.ts
git commit -m "test(frontend): cover apiFetch auth + refresh flow"
```

---

## Task 3: `auth.test.ts` — JWT expiry and logout

**Files:**
- Create: `frontend/src/lib/auth.test.ts`

`isLoggedIn()` decodes a JWT's middle segment as base64 JSON and checks `exp` against `Date.now()`. `logout()` clears storage and redirects. Five tests cover the four `isLoggedIn` branches plus one `logout` case.

- [ ] **Step 1: Create the test file**

Create `frontend/src/lib/auth.test.ts`:

```ts
import { describe, it, expect, beforeEach } from "vitest";
import { isLoggedIn, logout } from "./auth";

// Build an unsigned JWT whose payload has the given exp (seconds since epoch).
// The signature segment is arbitrary — isLoggedIn only decodes the payload.
function makeJwt(expSeconds: number): string {
  const header = btoa(JSON.stringify({ alg: "HS256", typ: "JWT" }));
  const payload = btoa(JSON.stringify({ exp: expSeconds }));
  return `${header}.${payload}.signature`;
}

describe("isLoggedIn", () => {
  it("returns false when no token is stored", () => {
    expect(isLoggedIn()).toBe(false);
  });

  it("returns true for a token whose exp is in the future", () => {
    const future = Math.floor(new Date("2100-01-01").getTime() / 1000);
    localStorage.setItem("access_token", makeJwt(future));
    expect(isLoggedIn()).toBe(true);
  });

  it("returns false for a token whose exp is in the past", () => {
    const past = Math.floor(new Date("2000-01-01").getTime() / 1000);
    localStorage.setItem("access_token", makeJwt(past));
    expect(isLoggedIn()).toBe(false);
  });

  it("returns false for a malformed token", () => {
    localStorage.setItem("access_token", "not.a.jwt");
    expect(isLoggedIn()).toBe(false);
  });
});

describe("logout", () => {
  beforeEach(() => {
    Object.defineProperty(window, "location", {
      value: { href: "" },
      writable: true,
      configurable: true,
    });
  });

  it("clears both tokens and sets window.location.href to /", () => {
    localStorage.setItem("access_token", "a");
    localStorage.setItem("refresh_token", "b");

    logout();

    expect(localStorage.getItem("access_token")).toBeNull();
    expect(localStorage.getItem("refresh_token")).toBeNull();
    expect(window.location.href).toBe("/");
  });
});
```

- [ ] **Step 2: Run the tests**

Run (from `frontend/`):

```bash
npm test -- src/lib/auth.test.ts
```

Expected: 5 tests, all PASS, exit code 0.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/auth.test.ts
git commit -m "test(frontend): cover isLoggedIn JWT expiry and logout"
```

---

## Task 4: `navigation.test.tsx` — context provider and hook

**Files:**
- Create: `frontend/src/lib/navigation.test.tsx`

`NavigationProvider` holds the current pathname in React state, exposes `navigate(href)` which calls `window.history.pushState`, and listens for `popstate` to sync state on back/forward. Three tests cover initial value, navigate, and popstate.

- [ ] **Step 1: Create the test file**

Create `frontend/src/lib/navigation.test.tsx`:

```tsx
import { describe, it, expect, vi, beforeEach } from "vitest";
import type { ReactNode } from "react";
import { act, renderHook } from "@testing-library/react";
import { NavigationProvider, useNavigation } from "./navigation";

function Wrapper({ children }: { children: ReactNode }) {
  return <NavigationProvider>{children}</NavigationProvider>;
}

describe("useNavigation", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/start");
  });

  it("path equals window.location.pathname on first render", () => {
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });
    expect(result.current.path).toBe("/start");
  });

  it("navigate(href) updates path and calls window.history.pushState", () => {
    const pushSpy = vi.spyOn(window.history, "pushState");
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });

    act(() => {
      result.current.navigate("/foo");
    });

    expect(result.current.path).toBe("/foo");
    expect(pushSpy).toHaveBeenCalledWith(null, "", "/foo");
  });

  it("a popstate event updates path to window.location.pathname", () => {
    const { result } = renderHook(() => useNavigation(), { wrapper: Wrapper });

    act(() => {
      window.history.replaceState(null, "", "/back-target");
      window.dispatchEvent(new PopStateEvent("popstate"));
    });

    expect(result.current.path).toBe("/back-target");
  });
});
```

- [ ] **Step 2: Run the tests**

Run (from `frontend/`):

```bash
npm test -- src/lib/navigation.test.tsx
```

Expected: 3 tests, all PASS, exit code 0.

If a test fails because of a React 19 / `@testing-library/react` version mismatch (error typically mentions `act` or `renderHook` from the wrong package), pin `@testing-library/react` to a version that supports React 19 (16.x or later) and re-run:

```bash
npm install --save-dev @testing-library/react@^16.3.0
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/lib/navigation.test.tsx
git commit -m "test(frontend): cover NavigationProvider and useNavigation"
```

---

## Task 5: Final verification

**Files:** No changes. Verification only.

- [ ] **Step 1: Run the full test suite**

Run (from `frontend/`):

```bash
npm test
```

Expected: all 16 tests across the three files PASS. Total runtime well under 5 seconds.

- [ ] **Step 2: Confirm the build still passes**

Run (from `frontend/`):

```bash
npm run build
```

Expected: static export succeeds with no TypeScript errors.

- [ ] **Step 3: No commit**

This task is verification-only. Nothing to commit.

---

## Self-Review Checklist

After all tasks complete:

- [ ] Spec coverage:
  - Vitest + jsdom + RTL harness — Task 1
  - 8 `apiFetch` cases — Task 2
  - 5 `auth` cases — Task 3
  - 3 `navigation` cases — Task 4
  - `npm test` + `npm run build` both clean — Task 5
- [ ] No tests require network, the real Go server, or a real browser.
- [ ] `frontend/package.json` has `test` and `test:watch` scripts; devDependencies include `vitest`, `jsdom`, `@testing-library/react`, `@vitejs/plugin-react`.
- [ ] All commits on the working branch; `npm test` runs in under 5 seconds.
