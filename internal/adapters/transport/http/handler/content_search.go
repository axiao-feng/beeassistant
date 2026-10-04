package handler

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

const (
	contentSearchMaxFileBytes      = 2 * 1024 * 1024
	contentSearchMaxResults        = 200
	contentSearchMaxMatchesPerFile = 20
)

type ContentSearchResult struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Text    string `json:"text"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mod_time"`
}

func SearchFileContentsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		baseDir, err := getWorkspaceDir()
		if err != nil {
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		query := strings.TrimSpace(c.Query("q"))
		if query == "" {
			Fail(c, http.StatusBadRequest, "搜索关键词不能为空")
			return
		}
		root, err := os.OpenRoot(baseDir)
		if err != nil {
			Fail(c, http.StatusInternalServerError, "failed to open workspace")
			return
		}
		results, err := searchWorkspaceContent(c.Request.Context(), root.FS(), query)
		_ = root.Close()
		if errors.Is(err, errSearchLimit) {
			Fail(c, http.StatusRequestEntityTooLarge, "搜索范围过大，请缩小知识库范围")
			return
		}
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			Fail(c, http.StatusInternalServerError, "搜索知识库内容失败")
			return
		}
		OK(c, results)
	}
}

func searchWorkspaceContent(ctx context.Context, filesystem fs.FS, query string) ([]ContentSearchResult, error) {
	queryLower := strings.ToLower(query)
	results := make([]ContentSearchResult, 0)
	visited := 0
	err := fs.WalkDir(filesystem, ".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		visited++
		if visited > maxSearchEntries {
			return errSearchLimit
		}
		if len(results) >= contentSearchMaxResults {
			return fs.SkipAll
		}
		relativePath := strings.TrimPrefix(filepath.ToSlash(path), "./")
		if entry.IsDir() && strings.Count(relativePath, "/") >= 10 {
			return fs.SkipDir
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		pathMatches := strings.Contains(strings.ToLower(entry.Name()), queryLower) || strings.Contains(strings.ToLower(relativePath), queryLower)
		if info.Size() > contentSearchMaxFileBytes {
			if pathMatches {
				results = append(results, ContentSearchResult{
					Name: entry.Name(), Path: relativePath, Line: 0,
					Text: "文件名或路径匹配", Size: info.Size(), ModTime: info.ModTime().Unix(),
				})
			}
			return nil
		}
		file, err := filesystem.Open(path)
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(file, contentSearchMaxFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) > contentSearchMaxFileBytes || !utf8.Valid(data) {
			if pathMatches {
				results = append(results, ContentSearchResult{
					Name: entry.Name(), Path: relativePath, Line: 0,
					Text: "文件名或路径匹配", Size: info.Size(), ModTime: info.ModTime().Unix(),
				})
			}
			return nil
		}
		matches := 0
		resultCountBeforeFile := len(results)
		for lineNumber, line := range strings.Split(string(data), "\n") {
			if !strings.Contains(strings.ToLower(line), queryLower) {
				continue
			}
			matches++
			results = append(results, ContentSearchResult{
				Name: entry.Name(), Path: relativePath, Line: lineNumber + 1,
				Text: searchSnippet(line), Size: info.Size(), ModTime: info.ModTime().Unix(),
			})
			if len(results) >= contentSearchMaxResults {
				return fs.SkipAll
			}
			if matches >= contentSearchMaxMatchesPerFile {
				break
			}
		}
		if len(results) == resultCountBeforeFile && pathMatches {
			results = append(results, ContentSearchResult{
				Name: entry.Name(), Path: relativePath, Line: 0,
				Text: "文件名或路径匹配", Size: info.Size(), ModTime: info.ModTime().Unix(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Path != results[j].Path {
			return results[i].Path < results[j].Path
		}
		return results[i].Line < results[j].Line
	})
	return results, nil
}

func searchSnippet(line string) string {
	line = strings.TrimSpace(line)
	runes := []rune(line)
	if len(runes) <= 500 {
		return line
	}
	return string(runes[:500]) + "…"
}
