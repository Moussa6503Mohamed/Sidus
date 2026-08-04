// Vitest stub for the "server-only" import guard. Next.js's own build replaces this package
// with a no-op in server bundles and throws only when a client bundle tries to import it; under
// plain Vitest neither bundler substitution runs, so the real package (an unconditional throw)
// would break every test that imports server-only code. This file is aliased in place of it.
export {};
