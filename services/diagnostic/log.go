package diagnostic

import (
	"bufio"
	"io"
	"sync"
	"time"
)

const RFC3339Milli = "2006-01-02T15:04:05.000Z07:00"

type Level int

const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

func defaultLevelF(lvl Level) bool {
	return true
}

type ServerLogger struct {
	mu      *sync.Mutex
	context []Field
	w       *bufio.Writer

	levelMu sync.RWMutex
	levelF  func(lvl Level) bool
}

func NewServerLogger(w io.Writer) *ServerLogger {
	var mu sync.Mutex
	return &ServerLogger{
		mu:     &mu,
		w:      bufio.NewWriter(w),
		levelF: defaultLevelF,
	}
}

// LevelF set on parent applies to self and any future children
func (l *ServerLogger) SetLevelF(f func(Level) bool) {
	l.levelMu.Lock()
	defer l.levelMu.Unlock()
	l.levelF = f
}

func (l *ServerLogger) With(ctx ...Field) Logger {
	l.mu.Lock()
	defer l.mu.Unlock()
	return &ServerLogger{
		mu: l.mu,
		// TODO: actually copy values not just append to previous context
		context: append(l.context, ctx...),
		w:       l.w,
		levelF:  l.levelF,
	}
}

func (l *ServerLogger) Error(msg string, ctx ...Field) {
	l.levelMu.RLock()
	logLine := l.levelF(ErrorLevel)
	l.levelMu.RUnlock()
	if logLine {
		l.Log(time.Now(), "error", msg, ctx)
	}
}

func (l *ServerLogger) Debug(msg string, ctx ...Field) {
	l.levelMu.RLock()
	logLine := l.levelF(DebugLevel)
	l.levelMu.RUnlock()
	if logLine {
		l.Log(time.Now(), "debug", msg, ctx)
	}
}

func (l *ServerLogger) Warn(msg string, ctx ...Field) {
	l.levelMu.RLock()
	logLine := l.levelF(WarnLevel)
	l.levelMu.RUnlock()
	if logLine {
		l.Log(time.Now(), "warn", msg, ctx)
	}
}

func (l *ServerLogger) Info(msg string, ctx ...Field) {
	l.levelMu.RLock()
	logLine := l.levelF(InfoLevel)
	l.levelMu.RUnlock()
	if logLine {
		l.Log(time.Now(), "info", msg, ctx)
	}
}

// TODO: actually care about errors?
func (l *ServerLogger) Log(now time.Time, level string, msg string, ctx []Field) {
	l.mu.Lock()
	defer l.mu.Unlock()

	writeTimestamp(l.w, now)
	l.w.WriteByte(' ')
	writeLevel(l.w, level)
	l.w.WriteByte(' ')
	writeMessage(l.w, msg)

	for _, f := range l.context {
		l.w.WriteByte(' ')
		f.WriteTo(l.w)
	}

	for _, f := range ctx {
		l.w.WriteByte(' ')
		f.WriteTo(l.w)
	}

	l.w.WriteByte('\n')

	l.w.Flush()
}

func writeTimestamp(w *bufio.Writer, now time.Time) {
	w.Write([]byte("ts="))
	// TODO: UTC?
	w.WriteString(now.Format(RFC3339Milli))
}

func writeLevel(w *bufio.Writer, lvl string) {
	w.Write([]byte("lvl="))
	w.WriteString(lvl)
}

func writeMessage(w *bufio.Writer, msg string) {
	w.Write([]byte("msg="))
	writeString(w, msg)
}
