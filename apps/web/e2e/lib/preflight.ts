/**
 * Stack preflight for the E2E harness (T-0025).
 *
 * The harness never starts or stops anything: the operator owns the `sidus-local-import` Compose
 * stack and the `apps/web` dev server, and keeps them running for their own manual testing
 * afterwards. All this does is prove both are reachable — including that Core's private dev TLS
 * chain actually verifies — and turn each failure into an instruction rather than a timeout
 * fifteen steps into a browser journey.
 */

/** Narrower than `NodeJS.ProcessEnv` (Next augments that with a required `NODE_ENV`) so callers
 * and tests can pass a plain object literal. */
export type E2eEnv = Readonly<Record<string, string | undefined>>;

export const DEFAULT_WEB_BASE_URL = "http://localhost:3001";
export const DEFAULT_CORE_BASE_URL = "https://127.0.0.1";

const PREFLIGHT_TIMEOUT_MS = 10_000;

export function webBaseUrl(env: E2eEnv = process.env): string {
  return (env.SIDUS_E2E_WEB_URL?.trim() || DEFAULT_WEB_BASE_URL).replace(/\/+$/, "");
}

export function coreBaseUrl(env: E2eEnv = process.env): string {
  return (env.SIDUS_E2E_CORE_URL?.trim() || DEFAULT_CORE_BASE_URL).replace(/\/+$/, "");
}

/** Node surfaces TLS chain failures as an error `cause` with one of these codes. Distinguishing
 * them from "connection refused" is the difference between "start the stack" and "set
 * NODE_EXTRA_CA_CERTS", so it is worth naming them explicitly. */
const TLS_ERROR_CODES = new Set([
  "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
  "SELF_SIGNED_CERT_IN_CHAIN",
  "DEPTH_ZERO_SELF_SIGNED_CERT",
  "CERT_HAS_EXPIRED",
  "ERR_TLS_CERT_ALTNAME_INVALID",
  "UNABLE_TO_GET_ISSUER_CERT_LOCALLY",
  "CERT_SIGNATURE_FAILURE",
]);

function causeCode(error: unknown): string | undefined {
  const cause = (error as { cause?: unknown } | null)?.cause;
  const code = (cause as { code?: unknown } | null)?.code;
  return typeof code === "string" ? code : undefined;
}

export function isTlsTrustFailure(error: unknown): boolean {
  const code = causeCode(error);
  return code !== undefined && TLS_ERROR_CODES.has(code);
}

export class PreflightError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "PreflightError";
  }
}

async function fetchJson(url: string): Promise<unknown> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), PREFLIGHT_TIMEOUT_MS);
  try {
    const response = await fetch(url, {
      headers: { Accept: "application/json" },
      redirect: "error",
      signal: controller.signal,
    });
    if (!response.ok) {
      throw new PreflightError(`${url} responded ${response.status}`);
    }
    return await response.json();
  } finally {
    clearTimeout(timeout);
  }
}

function isHealthy(body: unknown, service: string): boolean {
  const parsed = body as { service?: unknown; status?: unknown } | null;
  return parsed?.service === service && parsed?.status === "ok";
}

export async function checkWeb(env: E2eEnv = process.env): Promise<void> {
  const url = `${webBaseUrl(env)}/api/health`;
  let body: unknown;
  try {
    body = await fetchJson(url);
  } catch (error) {
    if (error instanceof PreflightError) throw error;
    throw new PreflightError(
      `the Sidus web app is not reachable at ${webBaseUrl(env)}.\n` +
        `Start it on the port this harness expects, pointed at the local HTTPS Core stack:\n` +
        `  $env:SIDUS_CORE_API_URL = "${coreBaseUrl(env)}"\n` +
        `  $env:NODE_EXTRA_CA_CERTS = "D:\\Sidus-private-content\\local-dev\\ca.pem"\n` +
        `  npm --prefix apps/web run dev -- --port 3001\n` +
        `See docs/e2e-harness.md.`,
    );
  }
  if (!isHealthy(body, "web")) {
    throw new PreflightError(`${url} did not report a healthy Sidus web app.`);
  }
}

export async function checkCore(env: E2eEnv = process.env): Promise<void> {
  const base = coreBaseUrl(env);
  let body: unknown;
  try {
    body = await fetchJson(`${base}/healthz`);
  } catch (error) {
    if (error instanceof PreflightError) throw error;
    if (isTlsTrustFailure(error)) {
      throw new PreflightError(
        `Core at ${base} is running but its TLS certificate does not verify (${causeCode(error)}).\n` +
          `This harness uses normal TLS verification — it never disables it. Point Node at the ` +
          `private dev CA before starting BOTH this run and the web dev server:\n` +
          `  $env:NODE_EXTRA_CA_CERTS = "D:\\Sidus-private-content\\local-dev\\ca.pem"\n` +
          `Regenerate the CA per docs/local-import-test-environment.md if it is missing or expired.`,
      );
    }
    throw new PreflightError(
      `Core is not reachable at ${base}.\n` +
        `Start the isolated local stack (it is left running afterwards — this harness never ` +
        `stops it):\n` +
        `  docker compose -f docker-compose.local-import.yml --env-file .env.local-import up -d\n` +
        `See docs/local-import-test-environment.md.`,
    );
  }
  if (!isHealthy(body, "core")) {
    throw new PreflightError(`${base}/healthz did not report a healthy Core.`);
  }
}

export async function preflight(env: E2eEnv = process.env): Promise<void> {
  await checkCore(env);
  await checkWeb(env);
}
