# Agent workflow

## System of record

| Need | File |
| --- | --- |
| Rules and architecture | `CLAUDE.md`, `docs/decisions.md` |
| Active work | `docs/tasks/active.md` |
| Completed/blocked work | `docs/tasks/history.md` |
| Cross-agent result | `docs/handoffs/` |

## Before work

1. Read `CLAUDE.md`, active task, decisions, relevant docs.
2. Confirm task status is `ready` or assigned to current agent.
3. Add assumptions, open questions, and implementation plan to active task.
4. Stop and mark `blocked` if a missing answer changes scope, data model, security, rights, cost, or external publication.

## During work

- One agent owns one task at once.
- Keep changes inside task scope.
- Write decisions with alternatives/reasons in `docs/decisions.md`.
- Do not encode unverified facts as requirements.

## Finish work

1. Run checks.
2. Create `docs/handoffs/T-XXXX.md` from template.
3. Move task into `docs/tasks/history.md` with status: `done` or `blocked`.
4. Commit only task files. Never stage unrelated local files.

## Status vocabulary

`backlog` → `ready` → `in_progress` → `review` → `done`

Use `blocked` for missing external decision, access, source rights, or reproducible failure.
