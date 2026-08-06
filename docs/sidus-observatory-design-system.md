# Sidus Observatory design system

Presentation-only visual system for `apps/web`, built in T-0017 from the user-supplied Claude
artifact "Sidus Design System & Screen Handoff" (`f3b99de0-8ba5-42d9-a9b4-080d86b9cc31`). No
Core/AI/BFF/database/business-rule change accompanies this document; see D-0019 and
`docs/handoffs/T-0017.md`.

## Direction

Light-first white + blue-ink identity. Premium, scientific, calm — no purple, orange, neon,
gradient, glassmorphism, space imagery, gaming visuals, or generic SaaS look. Dark mode is
navy-led (`#0A1626` base), never pure black or purple-tinted.

## Tokens — `apps/web/styles/tokens.css`

Single source of truth. Imported once, in `app/layout.tsx`. No hex value appears outside this
file (one intentional exception: `Logo.module.css`'s `.logo--inverse { color: #ffffff }` — the
inverse lockup is always white on a navy/brand ground regardless of site theme, so it cannot use
`--text-on-brand`, which flips in dark mode).

| Group | Tokens | Notes |
| --- | --- | --- |
| Type | `--font-display` / `--font-sans` / `--font-mono` | Fallback stacks only (Georgia serif / system-ui / Consolas) — no IBM Plex font files exist in the repo and none were fetched or committed per instruction. Swap in self-hosted Plex files via `next/font/local` later without touching call sites. |
| Color | `--surface-*`, `--border-*`, `--text-*`, `--brand-*`, `--focus-ring`, `--success/warning/danger/info/neutral-{fg,bg,border}` | Light values apply by default; `@media (prefers-color-scheme: dark)` and `:root[data-theme="dark"]` / `[data-theme="light"]` override identically, so OS preference and the explicit `ThemeToggle` choice agree. |
| Spacing | `--space-1` … `--space-20` | 4px base grid. |
| Radius | `--radius-xs/sm/md/lg/pill` | 12px (`lg`) is the largest corner anywhere in the product. |
| Border | `--border-hairline` (1px), `--border-emphasis` (2px) | 2px is reserved for MCQ selection/correctness and nothing else. |
| Elevation | `--shadow-1/2/3`, `--scrim` | Navy-tinted, low-alpha. No blur/glass. |

`prefers-reduced-motion: reduce` is a global rule in `tokens.css` that collapses every
animation/transition duration to ~0, so the two animated things in the app (button/loading
spinner, `Skeleton` shimmer) stop without each consumer opting in individually.

## Theme toggle

`components/theme/ThemeToggle.tsx` cycles **System → Light → Dark → System**, persisting the
explicit choice to `localStorage` under `sidus-theme` (and clearing it for "System"). It renders
"System" on the server and on first client render — the real stored value is only knowable after
mount — so hydration never mismatches; a `useEffect` reads `localStorage` once mounted.

`components/theme/theme-script.ts` exports a **constant** (no interpolated values) bootstrap
script inlined into `<head>` by `app/layout.tsx`, so the persisted theme applies before first
paint and there is no light/dark flash.

## Logo — `components/brand/Logo.tsx`

Original geometric navy delta-`A` with a triangular counter, followed by a six-bar asterisk —
read together as `A*`, the top Cambridge grade. Inline `currentColor` SVG, three variants
(`lockup` = mark + wordmark, `mark` = glyph only, `icon` = delta only, for favicon-scale use) and
three sizes (`sm`/`md`/`lg`). Never separate the asterisk from the `A`, add an outline/bevel/glow,
or use anything but navy (light) / `--text-primary` (dark) / white (`inverse` prop, fixed grounds
only).

## Central helpers — one place per cross-cutting concern

- **`lib/design/status.ts`** — the only place lifecycle-status label/icon/tone/border logic
  lives. `STATUS_VISUALS` covers all seven statuses in use across the app (`draft`, `pending`,
  `approved`, `verified`, `rejected`, `retired`, `expired`); every status surface
  (`components/ui/StatusBadge.tsx`, used by content sources, curriculum-map nodes, questions, and
  rubric versions) is a client of this map, so adding a state happens once. Colour is never the
  only signal: every entry also carries an icon shape and a border style (`dashed` for draft,
  `outline` for retired/expired), and retired/expired labels render struck through.
- **`lib/design/option-state.ts`** — pure function deriving one of six MCQ states
  (`default` / `selected` / `correct` / `incorrect` / `selected-incorrect` / `disabled`) plus its
  explicit text tag from `{ optionId, selectedOptionId, correctOptionId, isMarked, disabled }`.
  Used by Practice Mode's `question-list.tsx`; unit-tested in `option-state.test.ts` without
  rendering anything, per the file recommendation in the source artifact.

## MCQ option states (Practice Mode)

| State | Border | Key glyph | Text tag |
| --- | --- | --- | --- |
| `default` | 1px `--border-default` | outline | — |
| `selected` | 2px `--brand-primary` + tint fill | filled brand | "Selected" |
| `correct` | 2px `--success-fg` + success fill | filled green | "Correct answer" or "Your answer · correct" |
| `incorrect` | 1px default, no fill | outline, muted | "Not correct" |
| `selected-incorrect` | 2px `--danger-fg` + danger fill | filled red | "Your answer · incorrect" |
| `disabled` | 1px `--border-subtle`, sunken fill | muted | "Locked" |

Before marking, the option list is `role="radiogroup"` — labelled by the question prompt via
`aria-labelledby`, not a generic string — with a **roving tabindex**: the selected option (or the
first option, before anything is selected) is the sole `tabIndex={0}` stop, every other option is
`-1`. `ArrowRight`/`ArrowDown` and `ArrowLeft`/`ArrowUp` move focus to the next/previous option and
wrap; `Home`/`End` jump to the first/last option; each of these also updates the selection (matching
native radio-button behavior) before submission. `Space`/`Enter` select whichever option currently
has focus. `Tab` therefore enters the group once (at the roving stop) and leaves it once (at
"Submit answer"), per the WAI-ARIA APG radiogroup pattern — see `question-list.tsx`'s
`OptionGroup`. After marking (a result exists), the group switches to `role="list"` /
`role="listitem"`, every option becomes `disabled` (no `tabIndex`/roving logic applies), and no
answer, tag, or feedback is rendered before that point. A `sidus-visually-hidden` sentence
("You answered B. The correct answer is A. 0 of 1 mark.") is the first thing the `aria-live`
feedback region announces, ahead of the visible heading/explanations. Focused tests:
`question-list.test.tsx` (roving tabindex, all five key groups, focus movement, read-only
post-result state, no pre-submit disclosure).

## Mobile top nav (≤ 40rem)

`components/nav/nav.module.css` — the link strip (`.navlinks`) becomes a single-row, horizontally
scrolling region (`overflow-x: auto; flex-wrap: nowrap; min-width: 0`) instead of wrapping into a
tall multi-row header. Every other direct child of `.topnav` (brand, theme toggle, role chip, user
button) gets `flex-shrink: 0` in the same breakpoint so the scroll budget goes entirely to the
links; `.spacer` is hidden since `.navlinks` now grows to fill the remaining row width itself. No
link is hidden or truncated, every link stays reachable by keyboard, and the visible focus ring is
unaffected — only the overflow behavior changed. Header height stays a single row down to 390px.

## Token consistency

Every hard-coded pixel spacing/sizing value in `nav.module.css`, Practice Mode's
`styles.module.css`, and `Logo.module.css` was replaced with the closest `--space-*` token, or
`--border-hairline`/`--border-emphasis` for 1px/2px border and outline widths. Two deliberate,
documented carve-outs:

- **Border-compensated MCQ padding.** `.optionButton`'s default padding is
  `calc(var(--space-4) - var(--border-emphasis))`/`var(--space-4)`, and the selected/correct/
  selected-incorrect states are `calc(var(--space-4) - var(--border-emphasis) - var(--border-hairline))`/
  `calc(var(--space-4) - var(--border-hairline))`. This reproduces the original 14px/16px and
  13px/15px pixel values exactly, purely from tokens — a plain nearest-token snap on each value
  independently would have broken the padding-vs-border-width compensation and made the option box
  visibly resize by a few pixels when a question is marked.
- **Typography stays literal.** Font sizes, line-heights, and letter-spacing (in all three files)
  are not on the `--space-*` scale and no font-size token exists yet in `tokens.css`, so they were
  left as-is rather than force-mapped onto an unrelated token category. This is a known,
  intentionally scoped gap, not an oversight — a font-size scale is follow-up work if needed.

The one remaining literal color, `Logo.module.css`'s `.logo--inverse { color: #ffffff }`, stays as
the single documented exception described above (fixed-white lockup on a fixed navy/brand ground).

## Status system

Six lifecycle states (plus `expired`, used only by content sources) — see the table above. Status
badges are `<span>`, never a button; the icon is `aria-hidden`, the visible text is the accessible
name. `components/ui/StatusBadge.tsx` is the only renderer.

## Applied surfaces

Landing page (`app/page.tsx`), signed-in top nav + skip link + theme toggle + role chip
(`app/layout.tsx`, `components/nav/`), dashboard shell (`app/dashboard/page.tsx`), Practice Mode
(`app/dashboard/practice/*`), and all three editorial workspaces (sources, curriculum, questions —
`app/dashboard/editorial/*`) were restyled to consume tokens exclusively; no hex value remains in
any `apps/web` CSS/TSX file outside the one documented `Logo.module.css` exception. No product
behavior, route, API call, Clerk/auth flow, or BFF/Core boundary changed — see D-0019 and the
handoff for the itemized diff.

## Accessibility

- Visible focus ring (`outline: 2px solid var(--focus-ring); outline-offset: 2px`, token-derived
  via `--border-emphasis` in Practice Mode's own focus rule) is a global `:focus-visible` rule;
  never removed, never colour-only.
- Practice Mode's MCQ options are a full roving-tabindex `radiogroup` (arrow/Home/End/Space/Enter,
  single-stop `Tab` entry and exit) — see "MCQ option states" above.
- `prefers-reduced-motion: reduce` stops both animated things in the app.
- Every status/correctness signal pairs colour with an icon shape and/or border style and a text
  label — verified by `StatusBadge.test.tsx` and the MCQ result assertions in
  `question-list`'s consumer test (`workspace.test.tsx`).
- Responsive: existing breakpoints were preserved and retested at desktop width and 390px
  (`@media (max-width: 24.375rem)` in Practice Mode; existing `40rem`/`900px` breakpoints in the
  editorial pages).

## Implementation mapping

| Path | Contains |
| --- | --- |
| `apps/web/styles/tokens.css` | All tokens, global reset, skip-link/visually-hidden utility classes, reduced-motion rule. |
| `apps/web/components/theme/` | `ThemeToggle.tsx`, `theme-script.ts` (bootstrap), tests. |
| `apps/web/components/brand/Logo.tsx` | The `A*` mark, three variants/sizes. |
| `apps/web/components/nav/` | `RoleChip.tsx`, shared `nav.module.css` used by `app/layout.tsx`'s top nav. |
| `apps/web/components/ui/` | `Button`, `Message`, `Skeleton`, `StatusBadge` (+ `icons.tsx` shared glyphs), each with its own CSS module. |
| `apps/web/lib/design/` | `status.ts`, `option-state.ts` and their unit tests. |
