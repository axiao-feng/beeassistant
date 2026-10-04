package memory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"fkteams/internal/runtime/atomicfile"
)

const (
	extractionProgressFile       = ".extraction-progress.json"
	maxProgressFileBytes   int64 = 1 << 20
)

type sessionProgress struct {
	Offset         int       `json:"offset"`
	LastExtractAt  time.Time `json:"last_extract_at,omitempty"`
	LastAccessedAt time.Time `json:"last_accessed_at"`
}

type extractionProgress struct {
	Sessions map[string]sessionProgress `json:"sessions"`
}

func (m *Manager) progressPath() string {
	return filepath.Join(m.storeDir, extractionProgressFile)
}

func (m *Manager) loadProgress() {
	data, err := os.ReadFile(m.progressPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			// A damaged progress file must not prevent the assistant from starting.
			// The next successful extraction will rebuild it.
			return
		}
		return
	}
	if int64(len(data)) > maxProgressFileBytes {
		return
	}

	var state extractionProgress
	if err := json.Unmarshal(data, &state); err != nil {
		return
	}
	for sessionID, progress := range state.Sessions {
		if sessionID == "" || progress.Offset < 0 {
			continue
		}
		m.extractedOffsets[sessionID] = progress.Offset
		m.lastExtractTime[sessionID] = progress.LastExtractAt
		m.sessionAccess[sessionID] = progress.LastAccessedAt
	}
	if len(m.sessionAccess) <= maxTrackedSessions {
		return
	}

	for len(m.sessionAccess) > maxTrackedSessions {
		oldestID := ""
		var oldest time.Time
		for sessionID, accessedAt := range m.sessionAccess {
			if oldestID == "" || accessedAt.Before(oldest) {
				oldestID = sessionID
				oldest = accessedAt
			}
		}
		delete(m.sessionAccess, oldestID)
		delete(m.extractedOffsets, oldestID)
		delete(m.lastExtractTime, oldestID)
	}
}

// saveProgressLocked persists only extraction cursors, never conversation
// content. The caller must hold m.mu.
func (m *Manager) saveProgressLocked() error {
	state := extractionProgress{Sessions: make(map[string]sessionProgress, len(m.sessionAccess))}
	for sessionID, accessedAt := range m.sessionAccess {
		state.Sessions[sessionID] = sessionProgress{
			Offset:         m.extractedOffsets[sessionID],
			LastExtractAt:  m.lastExtractTime[sessionID],
			LastAccessedAt: accessedAt,
		}
	}
	if len(state.Sessions) == 0 {
		if err := os.Remove(m.progressPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.storeDir, 0755); err != nil {
		return err
	}
	return atomicfile.WriteFile(m.progressPath(), append(data, '\n'), 0600)
}
