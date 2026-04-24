# Editable SEO Description Prompt — Design

## Problem

The editor AI system prompt is editable from `/admin/settings` (setting key `ai_system_prompt`, wired through `internal/ai/handler.go`). The SEO meta-description system prompt is not — it lives as a hardcoded Go `const descriptionSystemPrompt` in `internal/pages/description.go:87`. Tweaking description style or tone currently requires a code change and redeploy.

## Goal

Make the SEO description prompt editable from the existing settings page, using the same pattern as `ai_system_prompt`. When unset, fall back to the current hardcoded default so behavior is unchanged out of the box.

## Non-Goals

- No per-page prompts. One global prompt.
- No prompt-versioning or history.
- No UI changes beyond adding the new field (no tabs, no separate sub-page).
- No change to the description generation model, token budget, or fallback-to-content-snippet behavior.

## Design

### Data

One new settings key: `ai_description_prompt`. Stored in the existing `settings` key/value table. **No migration required** — the table accepts arbitrary keys.

Empty / whitespace-only user value means "use the default." The default is the current prompt string, exported from Go as `pages.DefaultDescriptionPrompt`.

### Backend

**`internal/pages/description.go`:**
- Rename `descriptionSystemPrompt` → `DefaultDescriptionPrompt` (exported) so the settings handler can include it in GET responses without duplicating the string.
- Add `(g *Generator) loadDescriptionPrompt() string` — mirror of `loadAPIKey()`. Returns the trimmed user-configured prompt, or empty string on error / unset.
- In `Generate()`, replace the hardcoded `System: descriptionSystemPrompt` with:
  ```go
  system := g.loadDescriptionPrompt()
  if system == "" {
      system = DefaultDescriptionPrompt
  }
  ```
  and pass `system` to `ClaudeRequest.System`.

**`internal/settings/handler.go`:**
- Extend `SettingsResponse`:
  - `AIDescriptionPrompt string` (JSON: `ai_description_prompt`) — current user-configured value (empty if unset).
  - `AIDescriptionPromptDefault string` (JSON: `ai_description_prompt_default`) — always populated with `pages.DefaultDescriptionPrompt`.
- Extend `SettingsRequest` with `AIDescriptionPrompt *string` (JSON: `ai_description_prompt`).
- Add `"ai_description_prompt"` to the `IN (...)` clause of the GET query and to the settings map.
- Add an `upsert` branch in `Update()` for `ai_description_prompt`.
- In `Get()`, unconditionally set `resp.AIDescriptionPromptDefault = pages.DefaultDescriptionPrompt` before returning.

Because `internal/settings` importing `internal/pages` risks an import cycle (pages may in future import settings-related helpers), we keep the dependency one-way: `settings` imports `pages` only for the constant. Verify no cycle exists at build time; if one appears, move `DefaultDescriptionPrompt` to a leaf package such as `internal/pages/prompts` or inline it in a new `internal/ai/prompts` package shared by both.

### Frontend

**`frontend/src/app/admin/settings/page.tsx`:**
- Add state: `descriptionPrompt`, `descriptionPromptDefault`.
- On load, populate both from the GET response.
- Render a new section below the existing "AI System Prompt" textarea:
  - Label: `SEO Description Prompt` (matching existing uppercase style).
  - Textarea (4 rows; the default is a single sentence so shorter than the editor prompt field).
  - `placeholder={descriptionPromptDefault}` so users see the default when the field is empty.
  - Help text: "Used when generating meta descriptions. Leave empty to use the default."
  - Small "reset to default" link button that sets the textarea value to `""` (which on save falls back to default at generation time).
- On Save, include `ai_description_prompt: descriptionPrompt` in the PUT body.

### Data Flow

1. User opens `/admin/settings` → `GET /api/settings` → response includes `ai_description_prompt` (user value) and `ai_description_prompt_default` (const).
2. User edits textarea, clicks Save → `PUT /api/settings` with the new value (or empty string to reset).
3. On next editor save, `Generator.Generate()` runs: reads `ai_description_prompt` from settings; if non-empty, uses it; else uses `DefaultDescriptionPrompt`. All other generation behavior unchanged.

### Error Handling

- Invalid JSON on PUT: existing 400 response path.
- DB read failure in `loadDescriptionPrompt()`: returns empty string → falls back to default. Same shape as `loadAPIKey()`.
- AI API call failure: unchanged — falls back to `contentSnippet(content)`.
- Empty / whitespace user value: treated as "use default." Never send an empty `System` to the API.

### Testing

**`internal/pages/description_test.go`:**
- `TestGenerate_UsesCustomPromptWhenSet`: seed `ai_description_prompt` in the test DB, call `Generate()`, assert the stub `ClaudeClient` received the custom prompt as `req.System`.
- `TestGenerate_UsesDefaultWhenUnset`: leave the setting unset, call `Generate()`, assert `req.System == DefaultDescriptionPrompt`.
- `TestGenerate_UsesDefaultWhenEmpty`: set `ai_description_prompt` to `""`, assert default is used.

**`internal/settings/handler_test.go`:**
- Extend `TestUpdateAndGet` to include a non-empty `AIDescriptionPrompt` in the request and assert round-trip.
- Add `TestGet_IncludesDescriptionPromptDefault`: fresh DB, GET returns `AIDescriptionPromptDefault == pages.DefaultDescriptionPrompt`.

No new frontend tests — the settings page has no existing test harness, and the change is a straightforward textarea binding.

## Risks / Considerations

- **Prompt misuse.** A user could configure a prompt that produces invalid descriptions (too long, wrong format). Existing `postprocess()` already trims to 160 chars and strips quotes, so output is bounded regardless of prompt.
- **Silent drift.** If a user sets a prompt then we later improve `DefaultDescriptionPrompt`, the user's stored value wins. Acceptable — that's the point of an override. The UI shows the current default as placeholder so users can compare.
- **Import cycle (settings → pages).** If one appears, relocate `DefaultDescriptionPrompt` to a leaf package. Flagged above; verified at build.
