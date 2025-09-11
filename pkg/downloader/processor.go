package downloader

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/xpzouying/xiaohongshu-mcp/configs"
)

// ImageProcessor 图片处理器
type ImageProcessor struct {
	downloader *ImageDownloader
}

// NewImageProcessor 创建图片处理器
func NewImageProcessor() *ImageProcessor {
	return &ImageProcessor{
		downloader: NewImageDownloader(configs.GetImagesPath()),
	}
}

// ProcessImages 处理图片列表，返回本地文件路径
// 支持两种输入格式：
// 1. URL格式 (http/https开头) - 自动下载到本地
// 2. 本地文件路径 - 直接使用，支持 ~ 路径扩展
func (p *ImageProcessor) ProcessImages(images []string) ([]string, error) {
	var localPaths []string
	var urlsToDownload []string

	// 分离URL和本地路径
	for _, image := range images {
		if IsImageURL(image) {
			urlsToDownload = append(urlsToDownload, image)
		} else {
			// 处理本地路径，包括 ~ 扩展和路径验证
			slog.Info("Processing local image path", "original", image)
			expandedPath, err := p.expandAndValidatePath(image)
			if err != nil {
				slog.Error("Failed to process local path", "path", image, "error", err)
				return nil, fmt.Errorf("invalid local path %s: %w", image, err)
			}
			slog.Info("Successfully processed local path", "original", image, "expanded", expandedPath)
			localPaths = append(localPaths, expandedPath)
		}
	}

	// 批量下载URL图片
	if len(urlsToDownload) > 0 {
		downloadedPaths, err := p.downloader.DownloadImages(urlsToDownload)
		if err != nil {
			return nil, fmt.Errorf("failed to download images: %w", err)
		}
		localPaths = append(localPaths, downloadedPaths...)
	}

	if len(localPaths) == 0 {
		return nil, fmt.Errorf("no valid images found")
	}

	return localPaths, nil
}

// expandAndValidatePath 扩展路径（处理 ~ 符号）并验证文件是否存在
func (p *ImageProcessor) expandAndValidatePath(path string) (string, error) {
	var expandedPath string
	
	// 处理 ~ 路径扩展
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		expandedPath = filepath.Join(homeDir, path[2:])
		slog.Info("Expanded tilde path", "original", path, "expanded", expandedPath)
	} else {
		expandedPath = path
	}

	// 转换为绝对路径（只有在不是已经绝对路径的情况下）
	var absPath string
	if filepath.IsAbs(expandedPath) {
		absPath = expandedPath
	} else {
		var err error
		absPath, err = filepath.Abs(expandedPath)
		if err != nil {
			return "", fmt.Errorf("failed to get absolute path: %w", err)
		}
	}

	slog.Info("Final absolute path", "path", absPath)

	// 验证文件是否存在
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return "", fmt.Errorf("file does not exist: %s", absPath)
	} else if err != nil {
		return "", fmt.Errorf("failed to check file: %w", err)
	}

	return absPath, nil
}
