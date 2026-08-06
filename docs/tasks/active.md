# Active tasks

## T-0017 — Sidus Observatory visual system and responsive frontend polish

**Status:** review
**Owner:** Claude Code agent, 2026-08-06

### Scope

Apply the "Sidus Observatory" visual design system (light-first white + blue-ink, dark-mode
navy, `#14508C` light accent / `#6FA8E8` dark accent, IBM Plex Serif/Sans/Mono, 12px max radius,
original geometric `A*` delta-A + six-bar-asterisk logo) to `apps/web` presentation only. Source:
user-provided Claude artifact `f3b99de0-8ba5-42d9-a9b4-080d86b9cc31` ("Sidus Design System &
Screen Handoff"), fetched and read in full for exact tokens/components/specs.

In scope: design tokens (`styles/tokens.css`), theme toggle with persisted preference, `Logo`
component (inline SVG, no external asset), `lib/design/status.ts` and
`lib/design/option-state.ts` central helpers, `components/ui/*` presentation primitives,
restyle of landing page, dashboard shell/nav, Practice Mode (6 MCQ option states, non-color-only
correctness), editorial sources/curriculum/questions workspaces (6 lifecycle statuses with
label+icon+border, never color-only). Reduced motion, visible focus, keyboard flow, SR labels,
responsive desktop + 390px.

Out of scope (per instruction): no real educational content, no fake metrics/prices/timers, no
Exam Mode, no new API routes/DB changes/dependencies, no change to Clerk/auth behavior, Core/BFF
security boundaries, or existing learner/editorial interaction logic (only ARIA/presentation
refinement where required by the accessibility requirements above).

### Assumptions

- No IBM Plex font files exist in the repo and none may be fetched/committed per instruction —
  `--font-display`/`--font-sans`/`--font-mono` tokens use the documented fallback stacks
  (Georgia/serif, system-ui/Segoe UI, Consolas/SFMono) directly; no `next/font/local` call is
  added since there is no local font asset to point it at.
- "Do not change protected files" is read as: no edits to `services/core`, `services/ai`,
  `packages/shared` runtime contracts, migrations, or `lib/editorial|learner` request/permission
  logic — only `apps/web` presentation (components, CSS, page markup) and its own tests/docs.
- Where the design system requires an ARIA/semantic change for accessibility (e.g. marked MCQ
  options moving from an interactive control to a read-only list item) this is treated as
  in-scope presentation/accessibility work, not a change to product behavior, since the learner
  action (select once, submit once, view feedback) is unchanged.

### Plan

1. Foundation: tokens.css, ThemeToggle, Logo, status.ts, option-state.ts, ui/ primitives.
2. Apply to layout/landing/dashboard shell, Practice Mode, three editorial workspaces.
3. Add/extend focused tests (theme toggle, status rendering, MCQ states). Update only where
   changed semantics require it.
4. Docs: `docs/sidus-observatory-design-system.md`, this entry, handoff, decision log.
5. Validation: web vitest/typecheck/build, Core go build/vet/test, Python tests, `packages/shared`
   strict typecheck, compose config check, `git diff --check`, staged-content/secret audit.

### Open questions

None blocking — design source was fully machine-readable; no missing detail required guessing.

### Handoff

Implementation complete; full validation results, changed-file list, and known gaps are in
`docs/handoffs/T-0017.md`. See `docs/sidus-observatory-design-system.md` for the system itself
and D-0019 in `docs/decisions.md` for the decision record. Left at `review` — not released, not
pushed, per instruction.
