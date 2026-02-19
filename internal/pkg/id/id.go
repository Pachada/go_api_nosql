package id

import (
	"crypto/rand"

	"github.com/oklog/ulid/v2"
)

// New generates a new ULID string. ULIDs are lexicographically sortable
// by creation time and safe for use as DynamoDB partition keys.
func New() string {
	return ulid.MustNew(ulid.Now(), rand.Reader).String()
}
