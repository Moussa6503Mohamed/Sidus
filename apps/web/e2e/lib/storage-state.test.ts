// @vitest-environment node
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { afterEach, describe, expect, it } from "vitest";
import {
  assertPrivateRoot,
  DEFAULT_MAX_STATE_AGE_HOURS,
  E2eAuthError,
  isE2eProfile,
  privateStateDir,
  requestedProfiles,
  requireStorageStatePath,
  storageStateMetaPath,
  storageStatePath,
  validateStorageState,
} from "./storage-state";

const REPO_ROOT = path.resolve("D:/Sidus");
const NOW = 1_800_000_000;

const temporaryDirs: string[] = [];

function temporaryDir(): string {
  const dir = mkdtempSync(path.join(os.tmpdir(), "sidus-e2e-state-"));
  temporaryDirs.push(dir);
  return dir;
}

function writeState(dir: string, state: unknown, meta?: unknown): void {
  writeFileSync(storageStatePath("admin", dir), JSON.stringify(state), "utf8");
  if (meta !== undefined) {
    writeFileSync(storageStateMetaPath("admin", dir), JSON.stringify(meta), "utf8");
  }
}

function signedInState(expires = NOW + 3600) {
  return { cookies: [{ name: "__client", value: "opaque", expires }], origins: [] };
}

/** Asserts the call fails closed with a specific reason. Written out rather than using an
 * asymmetric matcher inside `toThrow` so the failure message names the reason that was seen. */
function expectAuthError(run: () => unknown, reason: string): E2eAuthError {
  try {
    run();
  } catch (error) {
    expect(error).toBeInstanceOf(E2eAuthError);
    expect((error as E2eAuthError).reason).toBe(reason);
    return error as E2eAuthError;
  }
  throw new Error(`expected an E2eAuthError with reason "${reason}", but nothing was thrown`);
}

afterEach(() => {
  while (temporaryDirs.length > 0) {
    rmSync(temporaryDirs.pop() as string, { recursive: true, force: true });
  }
});

describe("profile parsing", () => {
  it("accepts only the three known profiles", () => {
    expect(isE2eProfile("admin")).toBe(true);
    expect(isE2eProfile("learner")).toBe(true);
    expect(isE2eProfile("unknown")).toBe(true);
    expect(isE2eProfile("Admin")).toBe(false);
    expect(isE2eProfile("root")).toBe(false);
  });

  it("requires every profile by default so a missing capture cannot silently shrink the run", () => {
    expect(requestedProfiles({})).toEqual(["admin", "learner", "unknown"]);
  });

  it("narrows only when explicitly asked", () => {
    expect(requestedProfiles({ SIDUS_E2E_PROFILES: "admin, learner" })).toEqual(["admin", "learner"]);
  });

  it("rejects an unknown profile name rather than ignoring it", () => {
    expect(() => requestedProfiles({ SIDUS_E2E_PROFILES: "admin,superuser" })).toThrow(/superuser/);
  });

  it("treats an empty value as unset rather than as an empty run", () => {
    expect(requestedProfiles({ SIDUS_E2E_PROFILES: "   " })).toEqual(["admin", "learner", "unknown"]);
  });

  it('drops every authenticated project only for the explicit "none" opt-out', () => {
    expect(requestedProfiles({ SIDUS_E2E_PROFILES: "none" })).toEqual([]);
  });
});

describe("private state location", () => {
  it("defaults to the private content directory outside the repository", () => {
    expect(privateStateDir({})).toBe(path.resolve("D:\\Sidus-private-content\\e2e"));
  });

  it("honours an override to another private directory", () => {
    expect(privateStateDir({ SIDUS_E2E_STATE_DIR: "E:/elsewhere/e2e" })).toBe(
      path.resolve("E:/elsewhere/e2e"),
    );
  });

  it("refuses a state directory inside the repository", () => {
    expect(() => assertPrivateRoot(path.join(REPO_ROOT, "apps/web/e2e"), REPO_ROOT, "admin")).toThrow(
      E2eAuthError,
    );
    expect(() => assertPrivateRoot(REPO_ROOT, REPO_ROOT, "admin")).toThrow(/inside the repository/);
  });

  it("allows a sibling directory whose name merely starts with the repository path", () => {
    expect(() => assertPrivateRoot("D:/Sidus-private-content/e2e", REPO_ROOT, "admin")).not.toThrow();
  });
});

describe("validateStorageState", () => {
  it("accepts a capture holding a live Clerk cookie", () => {
    const result = validateStorageState(signedInState(), "admin", {
      nowSeconds: NOW,
      savedAtSeconds: NOW - 60,
    });
    expect(result.cookies).toHaveLength(1);
  });

  it("accepts a browser-session cookie with no stored expiry", () => {
    const state = { cookies: [{ name: "__session", value: "opaque", expires: -1 }] };
    expect(() =>
      validateStorageState(state, "admin", { nowSeconds: NOW, savedAtSeconds: NOW - 60 }),
    ).not.toThrow();
  });

  it("rejects anything that is not a storage-state file", () => {
    expectAuthError(
      () => validateStorageState({ nope: true }, "admin", { nowSeconds: NOW, savedAtSeconds: NOW - 60 }),
      "malformed",
    );
    expectAuthError(
      () => validateStorageState(null, "admin", { nowSeconds: NOW, savedAtSeconds: NOW - 60 }),
      "malformed",
    );
  });

  it("rejects a capture with no Clerk cookie rather than running unauthenticated", () => {
    const state = { cookies: [{ name: "theme", value: "dark", expires: NOW + 3600 }] };
    expectAuthError(() => validateStorageState(state, "admin", { nowSeconds: NOW }), "not_signed_in");
  });

  it("rejects a capture whose Clerk cookies have all expired", () => {
    expectAuthError(() => validateStorageState(signedInState(NOW - 1), "admin", { nowSeconds: NOW }), "expired");
  });

  it("rejects a capture older than the age limit even when its cookies still look valid", () => {
    expectAuthError(
      () =>
        validateStorageState(signedInState(NOW + 86_400), "admin", {
          nowSeconds: NOW,
          savedAtSeconds: NOW - (DEFAULT_MAX_STATE_AGE_HOURS + 1) * 3600,
        }),
      "expired",
    );
  });

  it("accepts a capture within the age limit", () => {
    expect(() =>
      validateStorageState(signedInState(NOW + 86_400), "admin", {
        nowSeconds: NOW,
        savedAtSeconds: NOW - 3600,
      }),
    ).not.toThrow();
  });

  it("never puts a cookie value into the failure message", () => {
    const state = { cookies: [{ name: "__session", value: "super-secret-jwt", expires: NOW - 1 }] };
    const error = expectAuthError(
      () => validateStorageState(state, "admin", { nowSeconds: NOW, savedAtSeconds: NOW - 60 }),
      "expired",
    );
    expect(error.message).not.toContain("super-secret-jwt");
  });
});

describe("requireStorageStatePath", () => {
  it("returns the path for a valid capture", () => {
    const dir = temporaryDir();
    writeState(dir, signedInState(), { profile: "admin", savedAtSeconds: NOW - 60 });
    const resolved = requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }, {
      nowSeconds: NOW,
    });
    expect(resolved).toBe(storageStatePath("admin", dir));
  });

  it("fails closed with capture instructions when no state exists", () => {
    const dir = temporaryDir();
    const error = expectAuthError(
      () => requireStorageStatePath("learner", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }),
      "missing",
    );
    expect(error.message).toContain("e2e:auth");
  });

  it("fails closed on an unparseable state file", () => {
    const dir = temporaryDir();
    writeFileSync(storageStatePath("admin", dir), "{ not json", "utf8");
    expectAuthError(
      () => requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }),
      "malformed",
    );
  });

  it("fails closed on a stale capture recorded by the sidecar", () => {
    const dir = temporaryDir();
    writeState(dir, signedInState(NOW + 86_400), {
      profile: "admin",
      savedAtSeconds: NOW - (DEFAULT_MAX_STATE_AGE_HOURS + 2) * 3600,
    });
    expectAuthError(
      () => requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }, { nowSeconds: NOW }),
      "expired",
    );
  });

  it("fails closed when the capture-time sidecar is missing or damaged", () => {
    const dir = temporaryDir();
    writeState(dir, signedInState(NOW + 3600));
    expectAuthError(
      () => requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }, { nowSeconds: NOW }),
      "missing",
    );
    writeFileSync(storageStateMetaPath("admin", dir), "{ broken", "utf8");
    expectAuthError(
      () => requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }, { nowSeconds: NOW }),
      "malformed",
    );
  });

  it("fails closed when the capture-time sidecar names another profile", () => {
    const dir = temporaryDir();
    writeState(dir, signedInState(NOW + 3600), { profile: "learner", savedAtSeconds: NOW - 60 });
    expectAuthError(
      () => requireStorageStatePath("admin", REPO_ROOT, { SIDUS_E2E_STATE_DIR: dir }, { nowSeconds: NOW }),
      "malformed",
    );
  });

  it("refuses to read state from inside the repository", () => {
    expectAuthError(
      () =>
        requireStorageStatePath("admin", REPO_ROOT, {
          SIDUS_E2E_STATE_DIR: path.join(REPO_ROOT, "apps/web/e2e/.state"),
        }),
      "unsafe_location",
    );
  });
});
