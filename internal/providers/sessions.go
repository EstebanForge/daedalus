package providers

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

type ACPPersistedSessionInfo struct {
	SessionKey  string
	ProviderKey string
	WorkDir     string
	Command     string
	SessionID   string
	CreatedAt   string
	UpdatedAt   string
	Stale       bool
}

type ACPActiveSessionInfo struct {
	SessionKey  string
	ProviderKey string
	WorkDir     string
	SessionID   string
	StartedAt   time.Time
	LastUsedAt  time.Time
	ExpiresAt   time.Time
}

// ListPersistedACPSessions returns persisted ACP cache records for the provided workdir.
func ListPersistedACPSessions(workDir string) ([]ACPPersistedSessionInfo, error) {
	cachePath, err := resolveACPSessionCachePath(workDir)
	if err != nil {
		return nil, err
	}

	acpPersistenceMu.Lock()
	cache, err := readSessionCache(cachePath)
	acpPersistenceMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("read persisted ACP sessions: %w", err)
	}

	now := time.Now().UTC()
	infos := make([]ACPPersistedSessionInfo, 0, len(cache.Sessions))
	for key, record := range cache.Sessions {
		stale := true
		if updatedAt, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(record.UpdatedAt)); parseErr == nil {
			stale = now.Sub(updatedAt) > acpPersistedMaxAge
		}

		providerKey := strings.TrimSpace(record.ProviderKey)
		if providerKey == "" {
			providerKey = providerFromSessionKey(key)
		}

		infos = append(infos, ACPPersistedSessionInfo{
			SessionKey:  key,
			ProviderKey: providerKey,
			WorkDir:     record.WorkDir,
			Command:     record.Command,
			SessionID:   record.SessionID,
			CreatedAt:   record.CreatedAt,
			UpdatedAt:   record.UpdatedAt,
			Stale:       stale,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].ProviderKey != infos[j].ProviderKey {
			return infos[i].ProviderKey < infos[j].ProviderKey
		}
		if infos[i].UpdatedAt != infos[j].UpdatedAt {
			return infos[i].UpdatedAt > infos[j].UpdatedAt
		}
		return infos[i].SessionKey < infos[j].SessionKey
	})

	return infos, nil
}

// ListActiveACPSessions returns in-memory ACP sessions currently managed by this process.
func ListActiveACPSessions() []ACPActiveSessionInfo {
	acpSessionsMu.RLock()
	defer acpSessionsMu.RUnlock()

	infos := make([]ACPActiveSessionInfo, 0, len(acpSessions))
	for key, session := range acpSessions {
		if session == nil {
			continue
		}

		session.useMu.Lock()
		startedAt := session.startedAt
		lastUsedAt := session.lastUsedAt
		session.useMu.Unlock()

		expiresAt := lastUsedAt.Add(acpSessionIdleTimeout)
		maxAgeAt := startedAt.Add(acpSessionMaxAge)
		if maxAgeAt.Before(expiresAt) {
			expiresAt = maxAgeAt
		}

		infos = append(infos, ACPActiveSessionInfo{
			SessionKey:  key,
			ProviderKey: providerFromSessionKey(key),
			WorkDir:     session.Cwd,
			SessionID:   session.ID,
			StartedAt:   startedAt,
			LastUsedAt:  lastUsedAt,
			ExpiresAt:   expiresAt,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		if infos[i].ProviderKey != infos[j].ProviderKey {
			return infos[i].ProviderKey < infos[j].ProviderKey
		}
		if infos[i].WorkDir != infos[j].WorkDir {
			return infos[i].WorkDir < infos[j].WorkDir
		}
		return infos[i].SessionID < infos[j].SessionID
	})

	return infos
}

func providerFromSessionKey(sessionKey string) string {
	parts := strings.SplitN(sessionKey, ":", 2)
	if len(parts) == 0 {
		return strings.TrimSpace(sessionKey)
	}
	return strings.TrimSpace(parts[0])
}
