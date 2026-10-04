package handler

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"fkteams/internal/app/appdata"
	"fkteams/internal/runtime/atomicfile"

	"github.com/gin-gonic/gin"
)

const (
	backupMaxTotalBytes = 256 << 20
	backupMaxFileBytes  = 32 << 20
)

var backupDirectories = []string{
	"sessions",
	"workspace",
	"todos",
	"profile",
	"scheduler",
	"skills",
	"javascript",
	"share",
}

var backupFiles = []string{"config/config.toml"}

// ExportBackupHandler 导出本机个人数据为 ZIP 文件。
func ExportBackupHandler(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		temp, err := os.CreateTemp("", "beeteams-backup-*.zip")
		if err != nil {
			Fail(c, http.StatusInternalServerError, "创建备份文件失败")
			return
		}
		tempPath := temp.Name()
		defer os.Remove(tempPath)

		if err := writeBackupArchive(temp, root); err != nil {
			_ = temp.Close()
			Fail(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := temp.Close(); err != nil {
			Fail(c, http.StatusInternalServerError, "完成备份文件失败")
			return
		}

		c.Header("Content-Type", "application/zip")
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="beeteams-backup-%s.zip"`, time.Now().Format("20060102-150405")))
		c.Header("Cache-Control", "no-store")
		c.File(tempPath)
	}
}

// RestoreBackupHandler 从 ZIP 文件恢复本机个人数据。
func RestoreBackupHandler(root string) gin.HandlerFunc {
	return func(c *gin.Context) {
		file, header, err := c.Request.FormFile("backup")
		if err != nil {
			Fail(c, http.StatusBadRequest, "请选择备份 ZIP 文件")
			return
		}
		defer file.Close()
		if header.Size > backupMaxTotalBytes {
			Fail(c, http.StatusRequestEntityTooLarge, "备份文件超过大小限制")
			return
		}

		temp, err := os.CreateTemp("", "beeteams-restore-*.zip")
		if err != nil {
			Fail(c, http.StatusInternalServerError, "创建恢复临时文件失败")
			return
		}
		tempPath := temp.Name()
		defer os.Remove(tempPath)
		if _, err := io.Copy(temp, io.LimitReader(file, backupMaxTotalBytes+1)); err != nil {
			_ = temp.Close()
			Fail(c, http.StatusBadRequest, "读取备份文件失败")
			return
		}
		if err := temp.Close(); err != nil {
			Fail(c, http.StatusInternalServerError, "保存恢复临时文件失败")
			return
		}

		restored, err := restoreBackupArchive(tempPath, root)
		if err != nil {
			Fail(c, http.StatusBadRequest, err.Error())
			return
		}
		OK(c, gin.H{"restored": restored, "restart_required": true})
	}
}

func writeBackupArchive(output *os.File, root string) error {
	archive := zip.NewWriter(output)

	var total int64
	writeFile := func(fullPath, archivePath string, info os.FileInfo) error {
		if info.Size() > backupMaxFileBytes {
			return fmt.Errorf("备份文件 %s 超过大小限制", archivePath)
		}
		if total+info.Size() > backupMaxTotalBytes {
			return fmt.Errorf("备份数据超过大小限制")
		}
		input, err := os.Open(fullPath)
		if err != nil {
			return fmt.Errorf("读取备份文件 %s 失败: %w", archivePath, err)
		}
		defer input.Close()
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return fmt.Errorf("创建备份条目失败: %w", err)
		}
		header.Name = archivePath
		header.Method = zip.Deflate
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("写入备份条目失败: %w", err)
		}
		if _, err := io.Copy(writer, input); err != nil {
			return fmt.Errorf("压缩备份文件 %s 失败: %w", archivePath, err)
		}
		total += info.Size()
		return nil
	}

	for _, relative := range backupFiles {
		fullPath := filepath.Join(root, filepath.FromSlash(relative))
		info, err := os.Stat(fullPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("读取备份文件 %s 失败: %w", relative, err)
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if err := writeFile(fullPath, relative, info); err != nil {
			return err
		}
	}

	for _, directory := range backupDirectories {
		fullRoot := filepath.Join(root, directory)
		if _, err := os.Stat(fullRoot); os.IsNotExist(err) {
			continue
		}
		if err := filepath.Walk(fullRoot, func(fullPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.Mode()&os.ModeSymlink != 0 {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(root, fullPath)
			if err != nil {
				return err
			}
			return writeFile(fullPath, filepath.ToSlash(relative), info)
		}); err != nil {
			return fmt.Errorf("扫描备份目录 %s 失败: %w", directory, err)
		}
	}
	return archive.Close()
}

func restoreBackupArchive(archivePath, root string) (int, error) {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return 0, fmt.Errorf("备份文件不是有效的 ZIP 文件")
	}
	defer archive.Close()

	staging, err := os.MkdirTemp("", "beeteams-restore-staging-")
	if err != nil {
		return 0, fmt.Errorf("创建恢复目录失败")
	}
	defer os.RemoveAll(staging)

	seen := make(map[string]struct{})
	var total uint64
	for _, file := range archive.File {
		name, ok := cleanBackupPath(file.Name)
		if !ok || !isAllowedBackupPath(name) {
			return 0, fmt.Errorf("备份包含不允许恢复的路径: %s", file.Name)
		}
		if _, exists := seen[name]; exists {
			return 0, fmt.Errorf("备份包含重复路径: %s", name)
		}
		seen[name] = struct{}{}
		if file.FileInfo().IsDir() {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 || file.UncompressedSize64 > backupMaxFileBytes || total+file.UncompressedSize64 > backupMaxTotalBytes {
			return 0, fmt.Errorf("备份文件 %s 超过安全限制", name)
		}
		outputPath := filepath.Join(staging, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return 0, fmt.Errorf("创建恢复目录失败: %w", err)
		}
		input, err := file.Open()
		if err != nil {
			return 0, fmt.Errorf("读取备份条目 %s 失败: %w", name, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(input, backupMaxFileBytes+1))
		_ = input.Close()
		if readErr != nil || len(data) > backupMaxFileBytes {
			return 0, fmt.Errorf("读取备份条目 %s 失败", name)
		}
		if err := os.WriteFile(outputPath, data, 0600); err != nil {
			return 0, fmt.Errorf("写入恢复目录失败: %w", err)
		}
		total += uint64(len(data))
	}

	restored := 0
	for name := range seen {
		stagedPath := filepath.Join(staging, filepath.FromSlash(name))
		info, err := os.Stat(stagedPath)
		if err != nil || !info.Mode().IsRegular() {
			continue
		}
		data, err := os.ReadFile(stagedPath)
		if err != nil {
			return 0, fmt.Errorf("读取待恢复文件 %s 失败: %w", name, err)
		}
		mode := os.FileMode(0644)
		if name == "config/config.toml" {
			mode = 0600
		}
		if err := atomicfile.WriteFile(filepath.Join(root, filepath.FromSlash(name)), data, mode); err != nil {
			return 0, fmt.Errorf("恢复文件 %s 失败: %w", name, err)
		}
		restored++
	}
	return restored, nil
}

func cleanBackupPath(raw string) (string, bool) {
	name := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if name == "" || strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", false
	}
	clean := path.Clean(name)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(clean, ":") {
		return "", false
	}
	return clean, true
}

func isAllowedBackupPath(name string) bool {
	if name == "config/config.toml" {
		return true
	}
	for _, directory := range backupDirectories {
		if strings.HasPrefix(name, directory+"/") {
			return true
		}
	}
	return false
}

// DefaultBackupRoot 返回默认个人数据目录，供路由组合使用。
func DefaultBackupRoot() string {
	return appdata.Dir()
}
