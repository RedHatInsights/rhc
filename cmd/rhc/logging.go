package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	LogFilePath string
	pid         = os.Getpid()
)

type PIDHandlerOptions struct {
	// Level reports the minimum level to log.
	// Levels with lower levels are discarded.
	// If nil, the Handler uses [slog.LevelInfo].
	Level slog.Leveler
}

// groupOrAttrs holds either a group name or a list of slog.Attrs.
type groupOrAttrs struct {
	group string      // group name if non-empty
	attrs []slog.Attr // attrs if non-empty
}

type PIDHandler struct {
	opts PIDHandlerOptions
	goas []groupOrAttrs
	mu   *sync.Mutex
	out  io.Writer
}

func NewPIDHandler(out io.Writer, opts *PIDHandlerOptions) *PIDHandler {
	h := &PIDHandler{out: out, mu: &sync.Mutex{}}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	return h
}

func (h *PIDHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *PIDHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)

	// Write the time if present
	if !r.Time.IsZero() {
		buf = fmt.Append(buf, r.Time.Format(time.RFC3339))
	}

	// Include process ID
	buf = fmt.Appendf(buf, " PID=%d", pid)

	// Include log level
	buf = fmt.Append(buf, " ", r.Level)

	// Write the message
	buf = fmt.Append(buf, " ", r.Message)

	// Build group prefix from WithGroup calls
	groupPrefix := ""

	// Handle state from WithGroup and WithAttrs.
	goas := h.goas
	if r.NumAttrs() == 0 {
		// If the record has no Attrs, remove groups at the end of the list; they are empty.
		for len(goas) > 0 && goas[len(goas)-1].group != "" {
			goas = goas[:len(goas)-1]
		}
	}
	for _, goa := range goas {
		if goa.group != "" {
			if groupPrefix != "" {
				groupPrefix += "."
			}
			groupPrefix += goa.group
		} else {
			for _, a := range goa.attrs {
				buf = h.appendAttr(buf, a, groupPrefix)
			}
		}
	}
	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, a, groupPrefix)
		return true
	})
	buf = append(buf, '\n')
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf)
	return err
}

func (h *PIDHandler) appendAttr(buf []byte, a slog.Attr, groupPrefix string) []byte {
	// Resolve the Attr's value before doing anything else.
	a.Value = a.Value.Resolve()

	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return buf
	}

	// Handle groups by building a prefix
	if a.Value.Kind() == slog.KindGroup {
		attrs := a.Value.Group()
		// Ignore empty groups.
		if len(attrs) == 0 {
			return buf
		}
		// Build the prefix for nested attributes
		prefix := groupPrefix
		if a.Key != "" {
			if prefix != "" {
				prefix = prefix + "." + a.Key
			} else {
				prefix = a.Key
			}
		}
		// Recursively append each grouped attribute with the prefix
		for _, ga := range attrs {
			buf = h.appendAttr(buf, ga, prefix)
		}
		return buf
	}

	// Build the full key with group prefix
	key := a.Key
	if groupPrefix != "" {
		key = groupPrefix + "." + key
	}

	// Format the attribute based on its type
	switch a.Value.Kind() {
	case slog.KindString:
		// Quote string values, to make them easy to parse.
		buf = fmt.Appendf(buf, " %s=%q", key, a.Value.String())
	case slog.KindTime:
		// Write times in a standard way, without the monotonic time.
		buf = fmt.Appendf(buf, " %s=%s", key, a.Value.Time().Format(time.RFC3339Nano))
	default:
		buf = fmt.Appendf(buf, " %s=%v", key, a.Value)
	}
	return buf
}

func (h *PIDHandler) withGroupOrAttrs(goa groupOrAttrs) *PIDHandler {
	h2 := *h
	h2.goas = make([]groupOrAttrs, len(h.goas)+1)
	copy(h2.goas, h.goas)
	h2.goas[len(h2.goas)-1] = goa
	return &h2
}

func (h *PIDHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return h.withGroupOrAttrs(groupOrAttrs{group: name})
}

func (h *PIDHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	return h.withGroupOrAttrs(groupOrAttrs{attrs: attrs})
}

// configureFileLogging sets up file-based logging to the configured log file path.
// It returns a cleanup function that should be called to close the log file.
// If file logging cannot be established, it falls back to stderr.
func configureFileLogging(logLevel slog.Leveler) func() error {
	// Attempt to open the log file
	// This file path typically resolves to /var/log/rhc/rhc.log
	LogFilePath = filepath.Join(LogDir, LongName, LongName+".log")
	file, err := os.OpenFile(LogFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)

	var w io.Writer
	var cleanup func() error
	if err != nil {
		// Fall back to stderr if we can't open the log file
		w = os.Stderr
		slog.Warn("unable to open log file, falling back to stderr", "error", err, "path", LogFilePath)
		// Return a no-op cleanup function since we don't own stderr
		cleanup = func() error { return nil }
	} else {
		w = file
		// Return cleanup function that closes the file
		cleanup = func() error {
			if file != nil {
				return file.Close()
			}
			return nil
		}
	}

	// Create and set the default logger
	h := NewPIDHandler(w, &PIDHandlerOptions{
		Level: logLevel,
	})

	logger := slog.New(h)
	slog.SetDefault(logger)

	return cleanup
}
