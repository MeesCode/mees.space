# Frontend Tests — Design

## Problem

The frontend has no automated tests. `npm run build` (the only current check) exercises TypeScript and the static exporter but not behavior. The three files in `frontend/src/lib/` (`api.ts`, `auth.ts`, `navigation.tsx`) contain the behavioral logic most likely to silently regress on refactor: token injection and refresh, JWT expiry parsing, client-side navigation.

## Goal

Add a lightweight, non-browser test harness and cover the three `src/lib/` files. No component tests yet, no E2E.

## Non-Goals

- No component tests (`MarkdownRenderer`, `Sidebar`, `ContentTree`, etc.).
- No contract tests against the Go backend.
- No Playwright or real-browser E2E.
- No coverage thresholds; tests are value-driven, not percentage-driven.
- No CI workflow changes beyond adding an `npm test` script — CI wiring is a follow-up.

## Design

### Tooling

- **Vitest** — fast, Vite-native, zero-config for Next.js / React 19. Runs in Node.
- **jsdom** environment — provides `window`, `localStorage`, `document`, DOM events. No browser; runs in Node.
- **@testing-library/react** — only for `renderHook` and `render` used by `navigation.test.tsx`. The `api.ts` and `auth.ts` tests do not render React.

### Files to Add

**Config (frontend root):**
- `frontend/vitest.config.ts` — sets `test.environment = "jsdom"`, `test.setupFiles = ["./vitest.setup.ts"]`, and mirrors the TypeScript path alias `@/* → src/*`.
- `frontend/vitest.setup.ts` — per-test cleanup hooks: `afterEach(() => { localStorage.clear(); sessionStorage.clear(); vi.restoreAllMocks(); })`.

**Package.json changes:**
- Add devDependencies: `vitest`, `@testing-library/react`, `jsdom`, `@vitejs/plugin-react`.
- Add scripts: `"test": "vitest run"`, `"test:watch": "vitest"`.

**Test files (co-located with subjects):**
- `frontend/src/lib/api.test.ts`
- `frontend/src/lib/auth.test.ts`
- `frontend/src/lib/navigation.test.tsx`

### Test Cases

**`api.test.ts` — `apiFetch()`:**
1. No token in localStorage → request dispatched without `Authorization` header.
2. Token present → request dispatched with `Authorization: Bearer <token>`.
3. `options.headers` supplied by caller are preserved alongside the `Authorization` header.
4. Non-401 response → returned unchanged; no refresh attempt.
5. `401` with no refresh token → tokens cleared, `window.location.href` set to `/admin/login`, no retry.
6. `401` with refresh token, refresh succeeds → new tokens written to localStorage, original request retried once with new `Authorization`, retry response returned.
7. `401` with refresh token, refresh returns non-OK → tokens cleared, redirected to `/admin/login`.
8. `401` with refresh token, refresh throws (network error) → tokens cleared, redirected to `/admin/login`.

Mock strategy: `vi.stubGlobal("fetch", vi.fn())` with per-test `mockResolvedValueOnce` calls. `window.location` replaced via `Object.defineProperty(window, "location", { value: { href: "" }, writable: true })` in `beforeEach`.

**`auth.test.ts` — `isLoggedIn()` / `logout()`:**
1. `isLoggedIn()` with no token → `false`.
2. `isLoggedIn()` with a token whose `exp` is in the future → `true`.
3. `isLoggedIn()` with a token whose `exp` is in the past → `false`.
4. `isLoggedIn()` with a malformed token (e.g. `"not.a.jwt"`) → `false` (the `atob`/`JSON.parse` throws, caught by the existing `try/catch`).
5. `logout()` clears both `access_token` and `refresh_token` and sets `window.location.href` to `/`.

JWTs are hand-built in the test (header/payload base64-encoded, signature arbitrary) — no signing library needed; only `exp` is parsed.

**`navigation.test.tsx` — `NavigationProvider` + `useNavigation()`:**
1. Inside a provider, `useNavigation().path` initially equals `window.location.pathname`.
2. Calling `navigate("/foo")` updates `path` to `/foo` and invokes `window.history.pushState`.
3. Firing a `popstate` event (after JSDOM `window.location.pathname` is changed) updates `path` to the new pathname.

Uses `renderHook` from `@testing-library/react` with a wrapper that mounts `NavigationProvider`. `window.history.pushState` is spied via `vi.spyOn(window.history, "pushState")`.

### Data Flow / Mocking

- `fetch` is stubbed globally per-test via `vi.stubGlobal` and cleared in the setup file's `afterEach`.
- `localStorage` is the real JSDOM implementation; cleared in `afterEach`.
- `window.location` is replaced per-test where assertions on `href` are needed. JSDOM's default `window.location` is read-only for navigation side effects, so the shim is required.
- `Date.now` is not mocked — JWT expiry tests use static absolute timestamps (e.g. `2000-01-01` for expired, `2100-01-01` for valid) rather than relative-to-now.

### Error Handling

No new production-code error paths. Tests only exercise existing branches. A test that fails because JSDOM behavior changed across versions is a test fix, not a product fix.

### Running

- `npm test` — one-shot, what CI (when added) will call.
- `npm run test:watch` — interactive; reruns on save.

### Out of Scope / Future

- Component tests once a regression motivates them.
- A contract test that hits a locally-running Go server — deferred unless UI/backend drift becomes a concrete problem.
- Coverage reporting — add only when a specific file is under-tested and we want to track it.

## Risks / Considerations

- **JSDOM drift.** JSDOM does not perfectly mirror real browsers; `window.location` is the main sharp edge. Mitigated by keeping the three tests that touch `window.location` small and obvious, with clear comments on the shim.
- **Navigation test fragility.** `navigation.test.tsx` asserts on `window.history.pushState` side effects, which are intrinsic to the implementation. If this test becomes churny, drop it and rely on manual verification of the three pages that use the hook.
- **React 19 + Vitest compatibility.** React 19 is recent; check that `@testing-library/react` supports it in the version installed. If not, pin compatible versions.
