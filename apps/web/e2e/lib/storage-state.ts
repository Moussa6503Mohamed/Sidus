import { existsSync, readFileSync } from "node:fs";
import path from "node:path";

/**
 * Resolution and fail-closed validation for Playwright browser storage state (T-0025).
 *
 * A storage-state file holds live Clerk session cookies. It is credential material: it must
 * never be written into, read from, or copied into this repository, and its contents must never
 * be logged, printed, or embedded in a test artifact. Everything in this module therefore
 * works on *paths and shapes* only — it never returns, formats, or logs a cookie value.
 *
 * Pure Node (no Playwright import) so the Vitest suite can exercise it directly.
 */

/** Every profile the harness knows about. Each maps to one real Clerk user the operator signs
 * in as interactively; there is no synthetic, injected, or bypassed session anywhere. */
export const E2E_PROFILES = ["admin", "learner", "unknown"] as const;
export type E2eProfile = (typeof E2E_PROFILES)[number];

/** Default private root. Never inside this repository — see assertPrivateRoot. */
export const DEFAULT_PRIVATE_E2E_DIR = "D:\\Sidus-private-content\\e2e";

/** A captured state older than this is treated as expired even if its cookies still claim to be
 * valid: Clerk sessions are revocable server-side, so age is an independent fail-closed signal. */
export const DEFAULT_MAX_STATE_AGE_HOURS = 8;

/** Cookie names Clerk sets. At least one must be present for a file to count as a real signed-in
 * capture rather than an empty/garbage context. */
const CLERK_COOKIE_PATTERN = /^__(session|client|clerk)/;

/** Just the environment shape this module reads. Narrower than `NodeJS.ProcessEnv` (which Next
 * augments with a required `NODE_ENV`) so tests can pass a plain object literal. */
export type E2eEnv = Readonly<Record<string, string | undefined>>;

export type E2eAuthFailure =
  | "not_configured"
  | "unsafe_location"
  | "missing"
  | "malformed"
  | "not_signed_in"
  | "expired";

/** Typed, message-only failure. Carries no path contents and no cookie data. */
export class E2eAuthError extends Error {
  readonly reason: E2eAuthFailure;
  readonly profile: E2eProfile;

  constructor(reason: E2eAuthFailure, profile: E2eProfile, message: string) {
    super(message);
    this.name = "E2eAuthError";
    this.reason = reason;
    this.profile = profile;
  }
}

export interface StorageStateCookie {
  name: string;
  /** Playwright's unix-seconds expiry. `-1` means a browser-session cookie with no stored expiry. */
  expires: number;
}

export interface StorageStateFile {
  cookies: StorageStateCookie[];
}

export interface ValidateOptions {
  /** Injected in tests; defaults to the real clock. Unix seconds. */
  nowSeconds?: number;
  /** Injected in tests; defaults to DEFAULT_MAX_STATE_AGE_HOURS. */
  maxAgeHours?: number;
  /** Unix seconds the state was captured, from the sidecar file. */
  savedAtSeconds?: number;
}

export function isE2eProfile(value: string): value is E2eProfile {
  return (E2E_PROFILES as readonly string[]).includes(value);
}

/**
 * The directory storage state may live in. `SIDUS_E2E_STATE_DIR` can override the default, but
 * only to another location outside this repository — the override exists for a different private
 * drive/path, never to move credentials into version control.
 */
export function privateStateDir(env: E2eEnv = process.env): string {
  const configured = env.SIDUS_E2E_STATE_DIR?.trim();
  return configured ? path.resolve(configured) : path.resolve(DEFAULT_PRIVATE_E2E_DIR);
}

/**
 * Refuses any state directory inside repoRoot. Checked on both the read and the write path, so a
 * mistyped env var can never cause a session cookie file to be created next to tracked code.
 */
export function assertPrivateRoot(dir: string, repoRoot: string, profile: E2eProfile): void {
  const resolvedDir = path.resolve(dir);
  const resolvedRepo = path.resolve(repoRoot);
  const relative = path.relative(resolvedRepo, resolvedDir);
  const insideRepo = relative === "" || (!relative.startsWith("..") && !path.isAbsolute(relative));
  if (insideRepo) {
    throw new E2eAuthError(
      "unsafe_location",
      profile,
      "refusing to read or write browser storage state inside the repository: it holds live " +
        "Clerk session cookies. Point SIDUS_E2E_STATE_DIR at a private directory outside the repo.",
    );
  }
}

export function storageStatePath(profile: E2eProfile, dir: string): string {
  return path.join(dir, `storage-state.${profile}.json`);
}

/** Sidecar holding capture time only — never any part of the session. Kept separate so the
 * state file itself stays exactly the shape Playwright wrote. */
export function storageStateMetaPath(profile: E2eProfile, dir: string): string {
  return path.join(dir, `storage-state.${profile}.meta.json`);
}

function howToCapture(profile: E2eProfile): string {
  return (
    `Capture it interactively first:\n` +
    `  $env:SIDUS_E2E_PROFILE = "${profile}"; npm --prefix apps/web run e2e:auth\n` +
    `You sign in yourself in the browser window that opens — the harness never handles your ` +
    `password. See docs/e2e-harness.md.`
  );
}

/**
 * Validates an already-parsed state object. Fails closed: any doubt about whether the session is
 * real and current is an error, never a silent downgrade to an unauthenticated run.
 */
export function validateStorageState(
  state: unknown,
  profile: E2eProfile,
  options: ValidateOptions = {},
): StorageStateFile {
  const now = options.nowSeconds ?? Math.floor(Date.now() / 1000);
  const maxAgeHours = options.maxAgeHours ?? DEFAULT_MAX_STATE_AGE_HOURS;

  const candidate = state as { cookies?: unknown } | null;
  if (!candidate || typeof candidate !== "object" || !Array.isArray(candidate.cookies)) {
    throw new E2eAuthError(
      "malformed",
      profile,
      `stored browser state for profile "${profile}" is not a Playwright storage-state file. ` +
        howToCapture(profile),
    );
  }

  const cookies = candidate.cookies.filter(
    (cookie): cookie is StorageStateCookie =>
      typeof cookie === "object" &&
      cookie !== null &&
      typeof (cookie as StorageStateCookie).name === "string" &&
      typeof (cookie as StorageStateCookie).expires === "number",
  );

  const clerkCookies = cookies.filter((cookie) => CLERK_COOKIE_PATTERN.test(cookie.name));
  if (clerkCookies.length === 0) {
    throw new E2eAuthError(
      "not_signed_in",
      profile,
      `stored browser state for profile "${profile}" contains no Clerk session cookie, so the ` +
        `run would be unauthenticated. ` +
        howToCapture(profile),
    );
  }

  // A browser-session cookie (-1) carries no expiry of its own, so it can only be judged by the
  // sidecar age check below. A stored expiry that has passed is decisive on its own.
  const usable = clerkCookies.some((cookie) => cookie.expires === -1 || cookie.expires > now);
  if (!usable) {
    throw new E2eAuthError(
      "expired",
      profile,
      `every Clerk cookie in the stored browser state for profile "${profile}" has expired. ` +
        howToCapture(profile),
    );
  }

  if (options.savedAtSeconds === undefined || !Number.isFinite(options.savedAtSeconds)) {
    throw new E2eAuthError(
      "malformed",
      profile,
      `stored browser state for profile "${profile}" has no valid capture-time evidence, so its ` +
        `freshness cannot be proved. ` +
        howToCapture(profile),
    );
  }

  if (options.savedAtSeconds > now) {
    throw new E2eAuthError(
      "malformed",
      profile,
      `stored browser state for profile "${profile}" has a future capture time, so its freshness ` +
        `cannot be proved. ` +
        howToCapture(profile),
    );
  }

  const ageHours = (now - options.savedAtSeconds) / 3600;
  if (ageHours > maxAgeHours) {
    throw new E2eAuthError(
      "expired",
      profile,
      `stored browser state for profile "${profile}" was captured ${Math.floor(ageHours)}h ago ` +
        `(limit ${maxAgeHours}h) and is treated as stale — a Clerk session can be revoked ` +
        `server-side without the cookie changing. ` +
        howToCapture(profile),
    );
  }

  return { cookies };
}

function readSavedAtSeconds(profile: E2eProfile, dir: string): number {
  const metaPath = storageStateMetaPath(profile, dir);
  if (!existsSync(metaPath)) {
    throw new E2eAuthError(
      "missing",
      profile,
      `captured browser state for profile "${profile}" has no capture-time sidecar, so its ` +
        `freshness cannot be proved. ` +
        howToCapture(profile),
    );
  }
  try {
    const parsed = JSON.parse(readFileSync(metaPath, "utf8")) as {
      profile?: unknown;
      savedAtSeconds?: unknown;
    };
    if (parsed.profile !== profile || typeof parsed.savedAtSeconds !== "number") {
      throw new E2eAuthError(
        "malformed",
        profile,
        `captured browser state for profile "${profile}" has an invalid capture-time sidecar. ` +
          howToCapture(profile),
      );
    }
    return parsed.savedAtSeconds;
  } catch {
    throw new E2eAuthError(
      "malformed",
      profile,
      `captured browser state for profile "${profile}" has an unreadable capture-time sidecar. ` +
        howToCapture(profile),
    );
  }
}

/**
 * Resolves the storage-state path for profile and proves it is present, safely located, and
 * currently usable. Returns the path only — the caller hands it to Playwright, which reads it;
 * the contents never pass through harness code or output.
 */
export function requireStorageStatePath(
  profile: E2eProfile,
  repoRoot: string,
  env: E2eEnv = process.env,
  options: ValidateOptions = {},
): string {
  const dir = privateStateDir(env);
  assertPrivateRoot(dir, repoRoot, profile);

  const statePath = storageStatePath(profile, dir);
  if (!existsSync(statePath)) {
    throw new E2eAuthError(
      "missing",
      profile,
      `no captured browser state for profile "${profile}". ` + howToCapture(profile),
    );
  }

  let parsed: unknown;
  try {
    parsed = JSON.parse(readFileSync(statePath, "utf8"));
  } catch {
    throw new E2eAuthError(
      "malformed",
      profile,
      `captured browser state for profile "${profile}" could not be parsed. ` + howToCapture(profile),
    );
  }

  validateStorageState(parsed, profile, {
    ...options,
    savedAtSeconds: options.savedAtSeconds ?? readSavedAtSeconds(profile, dir),
  });

  return statePath;
}

/** Explicit "run nothing that needs a captured session". Lets the signed-out fail-closed suite
 * run on a machine that has never captured any state — the one case where zero authenticated
 * projects is correct rather than a silently shrunken run. */
export const NO_PROFILES = "none";

/**
 * The profiles this run must have state for. Defaults to all of them: a profile is dropped only
 * by an explicit, conscious `SIDUS_E2E_PROFILES` narrowing, never by a file happening to be
 * absent. Unknown names are rejected rather than ignored.
 */
export function requestedProfiles(env: E2eEnv = process.env): E2eProfile[] {
  const raw = env.SIDUS_E2E_PROFILES?.trim();
  if (!raw) return [...E2E_PROFILES];
  if (raw === NO_PROFILES) return [];
  const names = raw
    .split(",")
    .map((name) => name.trim())
    .filter((name) => name !== "");
  if (names.length === 0) return [...E2E_PROFILES];
  const unknown = names.filter((name) => !isE2eProfile(name));
  if (unknown.length > 0) {
    throw new Error(
      `SIDUS_E2E_PROFILES lists unknown profile(s): ${unknown.join(", ")}. ` +
        `Known profiles: ${E2E_PROFILES.join(", ")}.`,
    );
  }
  return names.filter(isE2eProfile);
}
