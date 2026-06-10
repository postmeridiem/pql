package planning

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"time"
)

// crockfordEnc is Crockford base32 (no padding) — the ULID alphabet. 16 bytes
// (128 bits) encode to 26 sortable characters.
var crockfordEnc = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// NewRecordID returns a ULID-shaped record id: a 48-bit millisecond timestamp
// in the high bytes followed by 80 random bits, rendered as 26 Crockford
// base32 characters. It is the collision-proof, underwater identity of a
// ticket (record_id) — generated locally with no coordinator, so two clones
// never produce the same id, while the human-facing label (ticket_id, T-NNN)
// stays free to be reconciled. The leading timestamp makes ids sort by
// creation time. Stdlib-only (crypto/rand), so no new dependency.
func NewRecordID() (string, error) {
	var b [16]byte
	// 48-bit millisecond timestamp in the top 6 bytes (big-endian), so
	// lexicographic order of the encoding tracks creation order.
	binary.BigEndian.PutUint64(b[0:8], uint64(time.Now().UnixMilli())<<16)
	if _, err := rand.Read(b[6:]); err != nil {
		return "", fmt.Errorf("planning: generate record id: %w", err)
	}
	return crockfordEnc.EncodeToString(b[:]), nil
}
