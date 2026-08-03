package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func findNode(m model, path string) *TreeNode {
	return m.nodeIndex[path]
}

func TestUpsertItemsPatchesSkeleton(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", ApproxTokens: 25}})
	m := newModel(root, Options{})

	node := findNode(m, "a.go")
	if node == nil {
		t.Fatal("skeleton node a.go not created")
	}
	if node.ApproxTokens != 25 {
		t.Fatalf("approx tokens = %d, want 25", node.ApproxTokens)
	}

	m.upsertItems([]Item{{
		Path:       "a.go",
		Content:    []byte("package a\n"),
		TokensFull: 100,
		TokensSig:  20,
		RankScore:  0.9,
	}})

	node = findNode(m, "a.go")
	if string(node.Content) != "package a\n" {
		t.Errorf("content not patched: %q", node.Content)
	}
	if node.TokensFull != 100 || node.TokensSig != 20 {
		t.Errorf("tokens not patched: full=%d sig=%d", node.TokensFull, node.TokensSig)
	}
	if node.ApproxTokens != 0 {
		t.Errorf("approx tokens not cleared: %d", node.ApproxTokens)
	}
	if node.RankScore != 0.9 {
		t.Errorf("rank not patched: %v", node.RankScore)
	}
}

func TestUpsertItemsKeepsContentOnRankPatch(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.go", Content: []byte("keep me"), TokensFull: 10}})
	m := newModel(root, Options{})

	m.upsertItems([]Item{{Path: "a.go", RankScore: 1.0}})

	node := findNode(m, "a.go")
	if string(node.Content) != "keep me" {
		t.Errorf("rank-only patch clobbered content: %q", node.Content)
	}
	if node.RankScore != 1.0 {
		t.Errorf("rank not set: %v", node.RankScore)
	}
	if node.TokensFull != 10 {
		t.Errorf("tokens clobbered: %d", node.TokensFull)
	}
}

func TestUpsertItemsInsertsNewNode(t *testing.T) {
	root := BuildTree([]Item{{Path: "a.txt"}})
	m := newModel(root, Options{})

	m.upsertItems([]Item{{Path: "sub/deep.go", Content: []byte("p"), TokensFull: 3}})

	node := findNode(m, "sub/deep.go")
	if node == nil {
		t.Fatal("new node not inserted")
	}
	if string(node.Content) != "p" || node.TokensFull != 3 {
		t.Errorf("new node not enriched: %+v", node)
	}
	if _, ok := m.nodeIndex["sub"]; !ok {
		t.Error("intermediate dir not indexed")
	}
}

func TestNodesMsgKeepsCursorStable(t *testing.T) {
	items := make([]Item, 0, 6)
	for _, p := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt"} {
		items = append(items, Item{Path: p})
	}
	m := newModel(BuildTree(items), Options{})
	m.stream = Stream{Nodes: make(chan NodesMsg, 16), Progress: make(chan ProgressMsg, 4)}
	m.cursor = 3 // on d.txt

	// Insert a new file that would shift rows if the cursor were index-based.
	updated, cmd := m.Update(NodesMsg{Items: []Item{{Path: "a/new.txt", Content: []byte("x"), TokensFull: 1}}})

	mm := updated.(model)
	if mm.node() == nil || mm.node().Path != "d.txt" {
		t.Fatalf("cursor drifted to %v, want d.txt", mm.node())
	}
	if cmd == nil {
		t.Fatal("expected listener re-arm cmd")
	}
}

func TestProgressMsgDoneStillDrains(t *testing.T) {
	// Done must mark the scan finished but keep draining the nodes channel:
	// removal messages can arrive after Done, and stopping on Done would drop
	// them from the buffered channel.
	nodes := make(chan NodesMsg, 1)
	defer close(nodes)
	nodes <- NodesMsg{Remove: []string{"a.txt"}}

	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	m.stream = Stream{Nodes: nodes}

	updated, cmd := m.Update(ProgressMsg{Done: true, Files: 42})
	mm := updated.(model)
	if !mm.scanDone {
		t.Error("scanDone not set on Done")
	}
	if cmd == nil {
		t.Fatal("expected re-arm cmd after Done so pending nodes drain")
	}

	msg := cmd()
	if nm, ok := msg.(NodesMsg); !ok || len(nm.Remove) != 1 || nm.Remove[0] != "a.txt" {
		t.Fatalf("got %#v, want NodesMsg removing a.txt", msg)
	}
}

func TestListenStreamInactiveIsNil(t *testing.T) {
	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	if cmd := m.listenStream(); cmd != nil {
		t.Error("inactive stream should not arm a listener")
	}
}

func TestListenStreamSelectsFirstMessage(t *testing.T) {
	nodes := make(chan NodesMsg, 1)
	nodes <- NodesMsg{Items: []Item{{Path: "a.txt"}}}
	defer close(nodes)

	m := newModel(BuildTree([]Item{{Path: "b.txt"}}), Options{})
	m.stream = Stream{Nodes: nodes}

	cmd := m.listenStream()
	if cmd == nil {
		t.Fatal("active stream should arm a listener")
	}
	msg := cmd()
	nm, ok := msg.(NodesMsg)
	if !ok || len(nm.Items) != 1 || nm.Items[0].Path != "a.txt" {
		t.Fatalf("got %#v, want NodesMsg with a.txt", msg)
	}
}

func TestUpdateStreamMsgChains(t *testing.T) {
	// A non-done ProgressMsg must re-arm the listener.
	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	m.stream = Stream{}

	updated, cmd := m.Update(ProgressMsg{Files: 5})
	mm := updated.(model)
	if mm.scanFiles != 5 {
		t.Errorf("scanFiles = %d, want 5", mm.scanFiles)
	}
	if mm.scanDone {
		t.Error("scanDone set prematurely")
	}
	if cmd != nil {
		t.Error("inactive stream: ProgressMsg should not re-arm")
	}
}

func TestListenerDrainsAfterChannelClosed(t *testing.T) {
	// The scan producer closes its channels when it returns; a closed channel
	// is immediately selectable, so the listener must keep draining the others
	// instead of stopping at the first ok=false. This mirrors the real race:
	// removal is buffered on Nodes while Err is closed by streamScan's defer.
	nodes := make(chan NodesMsg, 1)
	progress := make(chan ProgressMsg, 1)
	errCh := make(chan error, 1)
	nodes <- NodesMsg{Remove: []string{"a.txt"}}
	close(errCh)
	defer close(nodes)
	defer close(progress)

	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	m.stream = Stream{Nodes: nodes, Progress: progress, Err: errCh}

	cmd := m.listenStream()
	if cmd == nil {
		t.Fatal("active stream should arm a listener")
	}
	msg := cmd()
	nm, ok := msg.(NodesMsg)
	if !ok || len(nm.Remove) != 1 || nm.Remove[0] != "a.txt" {
		t.Fatalf("got %#v, want the buffered removal before the closed Err channel stops the drain", msg)
	}
}

// TestListenStreamStopsRearmingAfterAllClosed verifies the idle-behavior
// guarantee: once every scan channel closes (normal completion or
// cancellation), the model stops arming listener commands, so BubbleTea has
// nothing polling and the process sits idle.
func TestListenStreamStopsRearmingAfterAllClosed(t *testing.T) {
	nodes := make(chan NodesMsg, 1)
	progress := make(chan ProgressMsg, 1)
	errCh := make(chan error, 1)
	close(nodes)
	close(progress)
	close(errCh)

	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	m.stream = Stream{Nodes: nodes, Progress: progress, Err: errCh}

	// Drive the listener through each channel closure, exactly as Update does.
	cmd := m.listenStream()
	for cmd != nil {
		msg := cmd()
		mm, ok := msg.(streamClosedMsg)
		if !ok {
			t.Fatalf("got %#v, want streamClosedMsg", msg)
		}
		switch mm.channel {
		case streamNodesClosed:
			m.streamNodesClosed = true
		case streamProgressClosed:
			m.streamProgressClosed = true
		case streamErrClosed:
			m.streamErrClosed = true
		}
		cmd = m.listenStream()
	}
}

// TestUpdateClearsStreamListenerAfterClosure drives the full Update loop with
// closed channels and asserts the final command is nil: no re-arm, no polling.
func TestUpdateClearsStreamListenerAfterClosure(t *testing.T) {
	nodes := make(chan NodesMsg, 1)
	progress := make(chan ProgressMsg, 1)
	errCh := make(chan error, 1)
	close(nodes)
	close(progress)
	close(errCh)

	m := newModel(BuildTree([]Item{{Path: "a.txt"}}), Options{})
	m.stream = Stream{Nodes: nodes, Progress: progress, Err: errCh}

	for i := 0; i < 10; i++ {
		updated, cmd := m.Update(streamClosedMsg{channel: streamNodesClosed})
		m = updated.(model)
		if m.scanDone {
			if cmd != nil {
				t.Fatalf("after scanDone cmd = %v, want nil", cmd)
			}
			return
		}
		// Before all channels are known closed, the listener is re-armed to
		// drain the others; feed progress and err closures too.
		updated, _ = m.Update(streamClosedMsg{channel: streamProgressClosed})
		m = updated.(model)
		updated, _ = m.Update(streamClosedMsg{channel: streamErrClosed})
		m = updated.(model)
		if cmd != nil && i > 4 {
			t.Fatalf("listener still armed before full closure: %v", cmd)
		}
	}
	t.Fatal("scanDone never reached with all channels closed")
}

// ensure tea is imported even when assertions are trimmed.
var _ tea.Msg = NodesMsg{}
