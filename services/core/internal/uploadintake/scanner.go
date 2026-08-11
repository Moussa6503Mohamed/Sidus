package uploadintake

import "context"

// MalwareScanner is intentionally an opaque-object boundary. Implementations receive a private
// object reference, never a learner URL. T-0030 ships no implementation and does not scan bytes.
// A future worker may call MarkScanClean only after this adapter returns clean.
type MalwareScanner interface {
	Scan(ctx context.Context, objectRef string, sha256 string) (clean bool, err error)
}
