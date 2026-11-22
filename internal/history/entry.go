// Package history records and reads sift's local render history.
//
// The package is a pure core: it takes directories and a clock as inputs and
// never reads environment variables, prints, or mutates global configuration.
// Command-layer code resolves XDG paths and supplies time.Now.
package history

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SchemaVersion is the version written into every history record.
const SchemaVersion = 1

// idSequence keeps IDs generated in one process unique within a second even
// when the random seed repeats. The four hex characters are a random start
// offset fixed at first use plus this counter, which cannot repeat until
// 65,536 IDs in the same process (far beyond one second's dumps).
var (
	idSequence atomic.Uint32
	idSeedOnce sync.Once
	idSeed     uint16
)

// Entry is one rendered-document record in history.jsonl.
type Entry struct {
	V      int    `json:"v"`
	ID     string `json:"id"`
	Ts     string `json:"ts"`
	Cmd    string `json:"cmd"`
	Root   string `json:"root"`
	Name   string `json:"name"`
	File   string `json:"file"`
	Bytes  int64  `json:"bytes"`
	Tokens int    `json:"tokens"`
	Files  int    `json:"files"`
	Style  string `json:"style"`
}

// NewID returns a sortable UTC timestamp ID with four hexadecimal
// characters appended. The suffix mixes a random start offset with a
// process-local counter so IDs generated in the same second do not repeat.
func NewID(now time.Time) string {
	idSeedOnce.Do(func() {
		var seed [2]byte
		if _, err := rand.Read(seed[:]); err == nil {
			idSeed = binary.BigEndian.Uint16(seed[:])
		}
	})
	value := idSeed + uint16(idSequence.Add(1))
	return fmt.Sprintf("%s-%04x", now.UTC().Format("20060102T150405Z"), value)
}
