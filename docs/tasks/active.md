# Active tasks

## T-0037 — Learner experience completion

**Status:** in progress
**Type:** Web frontend + accessibility

### Context

Sidus needs to finalize its learner experience before wider rollout.
The web shell must be offline-safe (caching the app shell but explicitly not private data).
The platform must support an Arabic/RTL scaffold to prepare for multi-language rollouts.
Tutor (Practice) and Test (Exam) modes need distinct rules to ensure different interaction models.
A full accessibility and mobile audit is required to ensure usability across devices.

### Goal

Complete the learner experience: distinct Tutor/Test rules, Arabic/RTL scaffold, offline-safe shell, and accessibility/mobile audit.

### Scope

- **Tutor/Test Rules:** Differentiate rules between Practice Mode (Tutor) and Exam Mode (Test).
- **Arabic/RTL Scaffold:** Set up Next.js internationalization (i18n) for Arabic (ar) or a layout that supports `dir="rtl"` dynamically, along with localized strings skeleton.
- **Offline-safe Shell:** Introduce a Service Worker to cache static assets/shell. Crucially, the service worker must NOT cache any private user data, API responses with answers, or any sensitive payloads.
- **Accessibility/Mobile Audit:** Fix contrast, keyboard navigation, screen reader labels, responsive layouts (max width 390px testing), and touch targets.

### Allowed files

- `apps/web/**`
- `docs/tasks/active.md`, `docs/handoffs/T-0037.md`

### Forbidden

- No live APIs/keys
- No PDFs/source content handling
- No question seeds
- No service worker caching of private data
- No human workflows or protected files
- No pushing to remote repositories

### Acceptance criteria

- Tutor and Test modes have distinct behavioral rules and visual cues.
- App shell can load offline via Service Worker, with zero private data cached.
- Application supports RTL layout and Arabic locale routing/context.
- Accessibility audit addressed (ARIA labels, focus states, color contrast).
- Mobile layout works flawlessly down to small device sizes.

### Validation

- Web tests (`npm --prefix apps/web run test`), typecheck (`npm --prefix apps/web run typecheck`), build (`npm --prefix apps/web run build`).
- `git diff --check` and secret audit.

### Stop condition

Implementation committed, release validation passed, release docs (handoff) committed. Leave review notes. Do not push.
