package handler

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	eventlog "fkteams/internal/adapters/storage/file/history"
	"fkteams/internal/domain/session"
	"fkteams/internal/runtime/log"

	"github.com/gin-gonic/gin"
)

const (
	defaultSessionSearchLimit = 30
	maxSessionSearchLimit     = 50
	maxSessionSearchMatches   = 3
	maxSessionSearchSnippet   = 240
)

type SessionSearchMatch struct {
	Type    string `json:"type"`
	At      string `json:"at,omitempty"`
	Snippet string `json:"snippet"`
}

type SessionSearchResult struct {
	SessionID string               `json:"session_id"`
	Title     string               `json:"title"`
	Matches   []SessionSearchMatch `json:"matches"`
}

// SearchSessionsHandler 按会话标题和聊天正文搜索历史会话。
func (rt *Runtime) SearchSessionsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		query := strings.TrimSpace(c.Query("q"))
		if utf8.RuneCountInString(query) < 2 {
			Fail(c, http.StatusBadRequest, "搜索关键词至少需要 2 个字符")
			return
		}

		limit := parseSessionSearchLimit(c.Query("limit"))
		records, err := rt.SessionService.List(c.Request.Context())
		if err != nil {
			FailError(c, err)
			return
		}

		results := make([]SessionSearchResult, 0, limit)
		for _, record := range records {
			if len(results) >= limit {
				break
			}
			if !session.ValidID(record.Metadata.ID) {
				continue
			}
			transcript, loadErr := eventlog.LoadSessionTranscriptRecords(filepath.Join(rt.HistoryDir, record.Metadata.ID))
			if loadErr != nil {
				if !errors.Is(loadErr, os.ErrNotExist) {
					log.Printf("failed to search session history: session=%s, err=%v", record.Metadata.ID, loadErr)
				}
				continue
			}
			matches := make([]SessionSearchMatch, 0, maxSessionSearchMatches)
			for _, item := range transcript {
				text := strings.TrimSpace(item.Event.Content)
				if text == "" {
					text = strings.TrimSpace(item.Event.Summary)
				}
				if snippet, ok := sessionSearchSnippet(text, query); ok {
					at := ""
					if !item.Event.At.IsZero() {
						at = item.Event.At.Format("2006-01-02T15:04:05Z07:00")
					}
					match := SessionSearchMatch{Type: string(item.Event.Type), At: at, Snippet: snippet}
					matches = append(matches, match)
					if len(matches) >= maxSessionSearchMatches {
						break
					}
				}
			}
			if len(matches) > 0 {
				results = append(results, SessionSearchResult{SessionID: record.Metadata.ID, Title: record.Metadata.Title, Matches: matches})
			}
		}

		OK(c, gin.H{"results": results, "total": len(results), "query": query})
	}
}

func parseSessionSearchLimit(raw string) int {
	if raw == "" {
		return defaultSessionSearchLimit
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return defaultSessionSearchLimit
	}
	if limit > maxSessionSearchLimit {
		return maxSessionSearchLimit
	}
	return limit
}

func sessionSearchSnippet(text, query string) (string, bool) {
	text = strings.TrimSpace(text)
	query = strings.TrimSpace(query)
	if text == "" || query == "" || !strings.Contains(strings.ToLower(text), strings.ToLower(query)) {
		return "", false
	}
	runes := []rune(text)
	if len(runes) <= maxSessionSearchSnippet {
		return text, true
	}
	return string(runes[:maxSessionSearchSnippet]) + "…", true
}
