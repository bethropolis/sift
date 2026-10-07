// appmode.go owns the app-window lifecycle: it counts the window's liveness
// stream and shuts the server down when the window is gone, so an app-mode
// server never outlives the UI it was opened for.
//
// Design note: the liveness channel carries no server-side heartbeat ticker.
// The handler simply blocks until the client goes away or the server starts
// shutting down, which keeps "an idle server does nothing" literally true:
// the only timers are the two deadlines below, both armed only while app mode
// is actually running.
package serve

import (
	"net/http"
	"sync"
	"time"
)

// appShutdownGrace is how long the window may stay disconnected before the
// server exits. Long enough to survive a reload or a brief network hiccup.
const appShutdownGrace = 15 * time.Second

// appLaunchWatchdog bounds how long a successful launch may go completely
// unseen before we conclude the window never loaded (bad browser, crashed
// window). It is cancelled by the first sign of life: the token exchange, a
// heartbeat connection, or any authenticated request (so a window that fell
// back to the password screen still keeps the server up).
const appLaunchWatchdog = 90 * time.Second

// Shutdown reasons logged when the app controller stops the server. These
// are the only shutdown paths that would otherwise exit silently, which
// makes a closed window indistinguishable from a crash.
const (
	shutdownWindowClosed    = "app window closed, shutting down"
	shutdownWindowNeverSeen = "app window never connected, shutting down"
)

// appController tracks the app window's connection count and owns the two
// shutdown deadlines. All state is behind one mutex; the clock and timer are
// injected so tests never sleep.
type appController struct {
	mu sync.Mutex
	// conns counts live liveness streams.
	conns int
	// liveness is enabled by arm() and is what makes a 1→0 transition
	// meaningful. It is never cleared by a connection event; only close()
	// disables it.
	liveness bool
	// seen records any sign of life from the window, which cancels the launch
	// watchdog.
	seen      bool
	stopFn    func() bool
	stopWatch func() bool
	shutdown  func(reason string)
	afterFunc func(time.Duration, func()) func() bool
}

func newAppController(shutdown func(reason string), afterFunc func(time.Duration, func()) func() bool) *appController {
	if afterFunc == nil {
		afterFunc = func(d time.Duration, f func()) func() bool {
			return time.AfterFunc(d, f).Stop
		}
	}
	return &appController{shutdown: shutdown, afterFunc: afterFunc}
}

// markAlive records that the app window reached the server in any way, which
// cancels the launch watchdog (the window clearly loaded) and cancels any
// pending graceful exit.
func (c *appController) markAlive() {
	c.mu.Lock()
	c.seen = true
	c.stopWatchLocked()
	stopGrace := c.stopFn
	c.stopFn = nil
	c.mu.Unlock()
	if stopGrace != nil {
		stopGrace()
	}
}

// addConn registers a liveness connection. It cancels any pending graceful
// exit (a reload reconnects inside the grace window) and satisfies the launch
// watchdog.
func (c *appController) addConn() {
	c.mu.Lock()
	c.seen = true
	c.stopWatchLocked()
	stopGrace := c.stopFn
	c.stopFn = nil
	c.conns++
	c.mu.Unlock()
	if stopGrace != nil {
		stopGrace()
	}
}

// dropConn deregisters a liveness connection and, when the last one leaves,
// arms the graceful-exit timer.
func (c *appController) dropConn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conns > 0 {
		c.conns--
	}
	if c.conns != 0 || !c.liveness || c.stopFn != nil {
		return
	}
	c.stopFn = c.afterFunc(appShutdownGrace, func() {
		c.mu.Lock()
		idle := c.conns == 0
		c.mu.Unlock()
		if idle && c.shutdown != nil {
			c.shutdown(shutdownWindowClosed)
		}
	})
}

// stopWatchLocked cancels the launch watchdog if armed. Callers hold the mutex.
func (c *appController) stopWatchLocked() {
	if c.stopWatch != nil {
		c.stopWatch()
		c.stopWatch = nil
	}
}

// arm prepares the controller for a real app launch: it enables liveness
// tracking and starts the watchdog that fires if the window never reports in.
func (c *appController) arm() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.liveness = true
	if c.seen || c.stopWatch != nil {
		return
	}
	c.stopWatch = c.afterFunc(appLaunchWatchdog, func() {
		c.mu.Lock()
		seen := c.seen
		c.mu.Unlock()
		if !seen && c.shutdown != nil {
			c.shutdown(shutdownWindowNeverSeen)
		}
	})
}

// close cancels any pending timers. Called during shutdown.
func (c *appController) close() {
	c.mu.Lock()
	stopGrace, stopWatch := c.stopFn, c.stopWatch
	c.stopFn, c.stopWatch = nil, nil
	c.liveness = false
	c.mu.Unlock()
	if stopGrace != nil {
		stopGrace()
	}
	if stopWatch != nil {
		stopWatch()
	}
}

// appAliveStream is the SSE body of GET /api/app/heartbeat. It stays open
// until the client disconnects or the server begins shutting down, and it
// never writes on a timer.
func appAliveStream(r *http.Request, w http.ResponseWriter, c *appController, done <-chan struct{}) {
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	// Tell the client to retry quickly if the stream ever drops, so a reload
	// reconnects well inside the grace window.
	if _, err := w.Write([]byte("retry: 2000\n\n")); err != nil {
		return
	}
	if flusher != nil {
		flusher.Flush()
	}

	c.addConn()
	defer c.dropConn()

	select {
	case <-r.Context().Done():
	case <-done:
	}
}
