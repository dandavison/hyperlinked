// Package ps provides print statement functions that emit OSC8 hyperlinks
// to the source location where the print was called. Clicking the output
// in a terminal that supports OSC8 will open your editor at that line.
//
// Suggested emoji prefixes for different operations:
//
//	⤴ Sent
//	⬅ Received
//	⬇ Written / Created
//	📡 Listening, long-polling
//	⚙️ State transition
//	🚀 Started
//	✅ Success
//	❌ Failure
//	🔄 Retry
//	🕐 Scheduled task execution
//	🟢 Good
//	🔴 Bad
//	🟡 In progress
package ps

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mattn/go-runewidth"
)

// LinkFormat controls the URL scheme for hyperlinks.
// Set via HYPERLINKED_FORMAT env var.
// Supported: "cursor" (default), "wormhole", "vscode"
var LinkFormat = getEnvDefault("HYPERLINKED_FORMAT", "cursor")

// Truncate controls whether output is truncated to terminal width.
// Set HYPERLINKED_NO_TRUNCATE=1 to disable.
var Truncate = os.Getenv("HYPERLINKED_NO_TRUNCATE") == ""

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// termWidth returns the terminal width, or 0 if it cannot be determined.
func termWidth() int {
	if cols := os.Getenv("HYPERLINKED_COLUMNS"); cols != "" {
		if width, err := strconv.Atoi(cols); err == nil && width > 0 {
			return width
		}
	}
	return 0
}

// truncateToWidth truncates text to fit within the given width.
// Preserves trailing newline if present. Uses "…" as ellipsis.
func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return text
	}

	// Preserve trailing newline
	hasNewline := strings.HasSuffix(text, "\n")
	if hasNewline {
		text = text[:len(text)-1]
	}

	visibleWidth := runewidth.StringWidth(text)
	if visibleWidth <= width {
		if hasNewline {
			return text + "\n"
		}
		return text
	}

	// Truncate to width-1 to leave room for ellipsis
	targetWidth := width - 1
	if targetWidth < 0 {
		targetWidth = 0
	}

	result := runewidth.Truncate(text, targetWidth, "…")

	if hasNewline {
		return result + "\n"
	}
	return result
}

var (
	startTime time.Time
	mu        sync.RWMutex
)

// StartTimer sets the start time for relative timestamps.
// Call this at the beginning of a test or program.
func StartTimer() {
	mu.Lock()
	defer mu.Unlock()
	startTime = time.Now()
}

// F prints with a millisecond timestamp prefix (like printf).
// The output is an OSC8 hyperlink to the call site.
func F(format string, args ...interface{}) {
	mu.RLock()
	start := startTime
	mu.RUnlock()

	ms := int64(0)
	if !start.IsZero() {
		ms = time.Since(start).Milliseconds()
	}

	text := fmt.Sprintf("[%5d] "+format, append([]interface{}{ms}, args...)...)
	fmt.Print(Hyperlink(text, 1))
}

// Ln prints with a millisecond timestamp prefix (like println).
// The output is an OSC8 hyperlink to the call site.
func Ln(msg string) {
	mu.RLock()
	start := startTime
	mu.RUnlock()

	ms := int64(0)
	if !start.IsZero() {
		ms = time.Since(start).Milliseconds()
	}

	text := fmt.Sprintf("[%5d] %s\n", ms, msg)
	fmt.Print(Hyperlink(text, 1))
}

// RelativeMs returns the milliseconds offset of t from the start time.
// Returns "now" for zero time, or the relative offset like "+1000" or "-500".
func RelativeMs(t time.Time) string {
	if t.IsZero() {
		return "now"
	}

	mu.RLock()
	start := startTime
	mu.RUnlock()

	if start.IsZero() {
		return t.Format(time.RFC3339Nano)
	}

	ms := t.Sub(start).Milliseconds()
	if ms >= 0 {
		return fmt.Sprintf("+%d", ms)
	}
	return fmt.Sprintf("%d", ms)
}

// Hyperlink wraps text in OSC8 escape codes linking to the caller's source location.
// skip is the number of stack frames to skip (0 = Hyperlink's caller, 1 = caller's caller, etc.)
// Truncates text to terminal width if Truncate is true.
func Hyperlink(text string, skip int) string {
	if Truncate {
		text = truncateToWidth(text, termWidth())
	}
	_, file, line, ok := runtime.Caller(skip + 1)
	if !ok {
		return text
	}
	url := FormatURL(file, line)
	return FormatOSC8(text, url)
}

// FormatOSC8 wraps text in OSC8 escape codes to create a clickable hyperlink.
func FormatOSC8(text, url string) string {
	const osc = "\x1b]"
	const st = "\x1b\\"
	return fmt.Sprintf("%s8;;%s%s%s%s8;;%s", osc, url, st, text, osc, st)
}

// FormatURL creates a URL for the given file and line based on LinkFormat.
func FormatURL(file string, line int) string {
	switch LinkFormat {
	case "wormhole":
		return fmt.Sprintf("http://wormhole:7117/file/%s:%d?land-in=editor", file, line)
	case "vscode":
		return fmt.Sprintf("vscode://file/%s:%d", file, line)
	case "cursor":
		fallthrough
	default:
		return fmt.Sprintf("cursor://file/%s:%d", file, line)
	}
}

// Stack prints the last n stack frames, each as a hyperlink to its source location.
// Skips runtime internals and starts from the caller of Stack.
func Stack(n int) {
	mu.RLock()
	start := startTime
	mu.RUnlock()

	ms := int64(0)
	if !start.IsZero() {
		ms = time.Since(start).Milliseconds()
	}

	// Skip 2: runtime.Callers + Stack
	pcs := make([]uintptr, n+2)
	got := runtime.Callers(2, pcs)
	if got == 0 {
		return
	}
	pcs = pcs[:got]

	width := 0
	if Truncate {
		width = termWidth()
	}

	frames := runtime.CallersFrames(pcs)
	i := 0
	for {
		frame, more := frames.Next()
		if i >= n {
			break
		}

		funcName := frame.Function
		if idx := lastIndex(funcName, '/'); idx >= 0 {
			funcName = funcName[idx+1:]
		}

		text := fmt.Sprintf("[%5d] #%d %s\n", ms, i, funcName)
		if Truncate && width > 0 {
			text = truncateToWidth(text, width)
		}
		url := FormatURL(frame.File, frame.Line)
		fmt.Print(FormatOSC8(text, url))

		i++
		if !more {
			break
		}
	}
}

func lastIndex(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}
