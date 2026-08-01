package tui

// DeltaInfo carries the git delta state the picker shows in its delta modal.
// It is nil (or DeltaInfo.Available is false) when the directory is not a git
// repository or no dump baseline has been recorded.
type DeltaInfo struct {
	RootDir    string
	FromHash   string
	FromMsg    string
	HeadHash   string
	HeadMsg    string
	Commits    []DeltaCommit // newest first; Checked selects how far back to go
	Files      []string      // changed files from FromHash..HEAD
	FilesToken int           // token estimate for the full-content strategy
	PatchToken int           // token estimate for the raw-patch strategy
	PatchLines int           // approximate line count of the raw patch
}

// DeltaCommit is one selectable commit shown in the delta modal.
type DeltaCommit struct {
	Short   string
	Subject string
	Checked bool
}

// DeltaStrategy selects how a delta dump is rendered.
type DeltaStrategy int

const (
	// DeltaFull dumps the full content of the changed files.
	DeltaFull DeltaStrategy = iota
	// DeltaPatch dumps the raw unified diff in a context_update block.
	DeltaPatch
)

// DeltaSelection describes a delta dump the user confirmed in the modal.
type DeltaSelection struct {
	Strategy  DeltaStrategy
	From      string
	To        string
	Clipboard bool
}

// newDeltaInfo returns a default DeltaInfo for the modal.
func newDeltaInfo() *DeltaInfo {
	return &DeltaInfo{}
}
