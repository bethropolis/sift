package tui

import tea "github.com/charmbracelet/bubbletea"

// Stream carries progressive scan data into the picker. Producers fill the
// channels in the background; the picker consumes them via tea.Cmd. All
// channels are closed once the scan finishes (after a final ProgressMsg with
// Done true is sent).
type Stream struct {
	// Nodes carries batches of enriched items that upsert into the skeleton
	// tree passed to RunStreaming.
	Nodes <-chan NodesMsg
	// Progress carries scan statistics for the footer.
	Progress <-chan ProgressMsg
	// Err carries a fatal scan error, after which no more messages arrive.
	Err <-chan error
}

// active reports whether any channel is wired up.
func (s Stream) active() bool {
	return s.Nodes != nil || s.Progress != nil || s.Err != nil
}

// NodesMsg is a batch of file items produced by the background walk. Items
// upsert into the skeleton tree; Remove lists skeleton paths the full walk
// later rejected, so the picker can drop files that will never enrich.
type NodesMsg struct {
	Items  []Item
	Remove []string
}

// ProgressMsg reports scan progress for the footer.
type ProgressMsg struct {
	Files     int
	Dirs      int
	Processed int
	Skipped   int
	Done      bool
}

// ErrMsg wraps a fatal scan error.
type ErrMsg struct {
	Err error
}

// streamClosedMsg reports that one scan channel was closed. The listener only
// stops once every channel is exhausted, so a closed channel must never abort
// the drain of messages still buffered on another.
type streamClosedMsg struct {
	channel int
}

const (
	streamNodesClosed int = iota
	streamProgressClosed
	streamErrClosed
)

// listenStream waits for the next message across all scan channels. It is
// re-armed after every message via tea.Batch, and only stops once every
// channel is closed (buffered values are delivered before a closed channel
// reports ok=false, so nothing in flight is lost).
func (m model) listenStream() tea.Cmd {
	if !m.stream.active() || (m.streamNodesClosed && m.streamProgressClosed && m.streamErrClosed) {
		return nil
	}
	return func() tea.Msg {
		nodes, progress, errCh := m.stream.Nodes, m.stream.Progress, m.stream.Err
		if m.streamNodesClosed {
			nodes = nil
		}
		if m.streamProgressClosed {
			progress = nil
		}
		if m.streamErrClosed {
			errCh = nil
		}
		// Prefer already-buffered messages over a concurrently closed channel.
		// A plain select treats a closed channel as permanently ready and can
		// therefore report stream closure before draining a message buffered on
		// another channel.
		if nodes != nil {
			select {
			case msg, ok := <-nodes:
				if !ok {
					return streamClosedMsg{channel: streamNodesClosed}
				}
				return msg
			default:
			}
		}
		if progress != nil {
			select {
			case msg, ok := <-progress:
				if !ok {
					return streamClosedMsg{channel: streamProgressClosed}
				}
				return msg
			default:
			}
		}
		if errCh != nil {
			select {
			case err, ok := <-errCh:
				if !ok {
					return streamClosedMsg{channel: streamErrClosed}
				}
				return ErrMsg{Err: err}
			default:
			}
		}
		select {
		case msg, ok := <-nodes:
			if !ok {
				return streamClosedMsg{channel: streamNodesClosed}
			}
			return msg
		case msg, ok := <-progress:
			if !ok {
				return streamClosedMsg{channel: streamProgressClosed}
			}
			return msg
		case err, ok := <-errCh:
			if !ok {
				return streamClosedMsg{channel: streamErrClosed}
			}
			return ErrMsg{Err: err}
		}
	}
}
