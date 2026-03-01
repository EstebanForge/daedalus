package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/EstebanForge/daedalus/internal/project"
)

func TestListPersistedACPSessions(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cachePath := project.ACPSessionsPath(workDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}

	cacheJSON := `{
  "version": 1,
  "sessions": {
    "codex:/tmp/project": {
      "providerKey": "codex",
      "workDir": "/tmp/project",
      "command": "codex-acp",
      "sessionId": "sess-1",
      "createdAt": "2026-02-28T10:00:00Z",
      "updatedAt": "2026-02-28T10:05:00Z"
    }
  }
}
`
	if err := os.WriteFile(cachePath, []byte(cacheJSON), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	records, err := ListPersistedACPSessions(workDir)
	if err != nil {
		t.Fatalf("list persisted sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}
	record := records[0]
	if record.ProviderKey != "codex" {
		t.Fatalf("unexpected provider key: %q", record.ProviderKey)
	}
	if record.SessionID != "sess-1" {
		t.Fatalf("unexpected session id: %q", record.SessionID)
	}
}

func TestListPersistedACPSessionsMarksStaleRecord(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	cachePath := project.ACPSessionsPath(workDir)
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("mkdir cache dir: %v", err)
	}

	oldTimestamp := time.Now().UTC().Add(-(acpPersistedMaxAge + time.Hour)).Format(time.RFC3339)
	cacheJSON := `{"version":1,"sessions":{"codex:/tmp/project":{"providerKey":"codex","workDir":"/tmp/project","command":"codex-acp","sessionId":"sess-old","createdAt":"` + oldTimestamp + `","updatedAt":"` + oldTimestamp + `"}}}`
	if err := os.WriteFile(cachePath, []byte(cacheJSON), 0o644); err != nil {
		t.Fatalf("write cache file: %v", err)
	}

	records, err := ListPersistedACPSessions(workDir)
	if err != nil {
		t.Fatalf("list persisted sessions: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}
	if !records[0].Stale {
		t.Fatalf("expected stale record, got %+v", records[0])
	}
}

func TestListActiveACPSessions(t *testing.T) {
	t.Parallel()
	t.Cleanup(CloseAllSessions)

	now := time.Now().UTC()
	acpSessionsMu.Lock()
	acpSessions["codex:/tmp/work"] = &acpSessionState{
		ID:         "sess-active",
		Cwd:        "/tmp/work",
		startedAt:  now.Add(-time.Minute),
		lastUsedAt: now,
	}
	acpSessionsMu.Unlock()

	records := ListActiveACPSessions()
	if len(records) != 1 {
		t.Fatalf("expected one active session, got %d", len(records))
	}
	record := records[0]
	if record.ProviderKey != "codex" {
		t.Fatalf("unexpected provider key: %q", record.ProviderKey)
	}
	if !strings.Contains(record.SessionKey, "codex:") {
		t.Fatalf("unexpected session key: %q", record.SessionKey)
	}
	if record.SessionID != "sess-active" {
		t.Fatalf("unexpected session id: %q", record.SessionID)
	}
}
