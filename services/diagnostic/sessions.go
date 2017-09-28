package diagnostic

import (
	"bytes"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/influxdata/kapacitor/uuid"
)

type SessionsStore interface {
	Create(w http.ResponseWriter, contentType string, tags []tag) *Session
	Delete(s *Session) error
	Each(func(*Session))
}

type sessionsStore struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*Session
}

func (kv *sessionsStore) Create(w http.ResponseWriter, contentType string, tags []tag) *Session {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	flusher, ok := w.(http.Flusher)
	var wf WriteFlusher = &noopWriteFlusher{w: w}
	if ok {
		wf = &httpWriteFlusher{w: w, f: flusher}
	}

	s := &Session{
		id:          uuid.New(),
		tags:        tags,
		w:           wf,
		contentType: contentType,
	}

	kv.sessions[s.id] = s

	return s
}

func (kv *sessionsStore) Delete(s *Session) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	if s == nil {
		return errors.New("session is nil")
	}

	delete(kv.sessions, s.id)

	return nil
}

func (kv *sessionsStore) Each(fn func(*Session)) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	for _, s := range kv.sessions {
		fn(s)
	}
}

type tag struct {
	key   string
	value string
}

type Session struct {
	mu sync.Mutex
	id uuid.UUID

	tags []tag

	buf         bytes.Buffer
	w           WriteFlusher
	contentType string
}

func (s *Session) Error(msg string, context, fields []Field) {
	if match(s.tags, msg, "error", context, fields) {
		s.Log(time.Now(), "error", msg, context, fields)
	}
}

func (s *Session) Warn(msg string, context, fields []Field) {
	if match(s.tags, msg, "warn", context, fields) {
		s.Log(time.Now(), "warn", msg, context, fields)
	}
}

func (s *Session) Debug(msg string, context, fields []Field) {
	if match(s.tags, msg, "debug", context, fields) {
		s.Log(time.Now(), "debug", msg, context, fields)
	}
}

func (s *Session) Info(msg string, context, fields []Field) {
	if match(s.tags, msg, "info", context, fields) {
		s.Log(time.Now(), "info", msg, context, fields)
	}
}

func (s *Session) Log(now time.Time, level, msg string, context, fields []Field) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch s.contentType {
	case "application/json":
		writeJSON(&s.buf, now, level, msg, context, fields)
	default:
		writeLogfmt(&s.buf, now, level, msg, context, fields)
	}
	s.w.Write(s.buf.Bytes())
	s.buf.Reset()
	s.w.Flush()
}

func match(tags []tag, msg, level string, context, fields []Field) bool {
	ctr := 0
Loop:
	for _, t := range tags {
		if t.key == "msg" && t.value == msg {
			ctr++
			continue Loop
		}
		if t.key == "lvl" && t.value == level {
			ctr++
			continue Loop
		}
		for _, c := range context {
			if c.Match(t.key, t.value) {
				ctr++
				continue Loop
			}
		}
		for _, f := range fields {
			if f.Match(t.key, t.value) {
				ctr++
				continue Loop
			}
		}
	}

	return len(tags) == ctr
}

type noopWriteFlusher struct {
	w http.ResponseWriter
}

func (h *noopWriteFlusher) Write(buf []byte) (int, error) {
	return h.w.Write(buf)
}
func (h *noopWriteFlusher) Flush() error {
	return nil
}

type httpWriteFlusher struct {
	w http.ResponseWriter
	f http.Flusher
}

func (h *httpWriteFlusher) Write(buf []byte) (int, error) {
	return h.w.Write(buf)
}
func (h *httpWriteFlusher) Flush() error {
	h.f.Flush()
	return nil
}
