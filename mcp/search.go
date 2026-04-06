package mcp

import (
	"AfdianToMarkdown/storage"
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Search 在数据目录中全文搜索关键词
// 支持按作者过滤，返回最多 maxResults 条结果
func Search(dataDir, query, author string, maxResults int) (*storage.SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("请提供搜索关键词")
	}

	// 确定搜索范围（作者列表）
	var authors []string
	if author != "" {
		// 通过 ListPosts 验证作者是否存在（内含路径遍历检查）
		_, err := storage.ListPosts(dataDir, author)
		if err != nil {
			return nil, fmt.Errorf("作者不存在：%s", author)
		}
		authors = []string{author}
	} else {
		var err error
		authors, err = storage.ListAuthors(dataDir)
		if err != nil {
			return nil, err
		}
	}

	resp := &storage.SearchResponse{
		Query: query,
	}
	queryLower := strings.ToLower(query)

	// 遍历所有作者的文件
	for _, a := range authors {
		authorDir := filepath.Join(dataDir, a)
		err := filepath.Walk(authorDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			// 跳过目录和非 .md 文件，跳过 .assets 目录
			if info.IsDir() {
				if info.Name() == ".assets" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(info.Name(), ".md") {
				return nil
			}

			// 计算相对路径
			relPath, err := filepath.Rel(dataDir, path)
			if err != nil {
				return nil
			}
			relPath = filepath.ToSlash(relPath)

			// 解析文章标题
			postInfo := storage.ParsePostInfo(info.Name(), "", "")

			// 逐行搜索
			searchFileForMatches(path, relPath, postInfo.Title, a, queryLower, maxResults, resp)

			return nil
		})
		if err != nil {
			continue
		}
	}

	return resp, nil
}

// searchFileForMatches 在单个文件中搜索匹配行，将结果追加到 resp
func searchFileForMatches(filePath, relPath, title, author, queryLower string, maxResults int, resp *storage.SearchResponse) {
	f, err := os.Open(filePath)
	if err != nil {
		return
	}
	defer f.Close()

	// 先读取所有行，便于提取上下文
	var lines []string
	scanner := bufio.NewScanner(f)
	// 增大缓冲区以处理超长行
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), queryLower) {
			resp.TotalCount++
			if len(resp.Results) >= maxResults {
				resp.Truncated = true
				continue
			}

			// 提取前后各 3 行上下文
			context := buildContext(lines, i)

			resp.Results = append(resp.Results, storage.SearchResult{
				FilePath:   relPath,
				Title:      title,
				Author:     author,
				LineNumber: i + 1, // 行号从 1 开始
				Context:    context,
			})
		}
	}
}

// buildContext 构建匹配行及前后各 3 行的上下文文本
func buildContext(lines []string, matchIndex int) string {
	start := matchIndex - 3
	if start < 0 {
		start = 0
	}
	end := matchIndex + 3
	if end >= len(lines) {
		end = len(lines) - 1
	}

	var sb strings.Builder
	for i := start; i <= end; i++ {
		lineNum := i + 1
		if i == matchIndex {
			sb.WriteString(fmt.Sprintf("> %d | %s\n", lineNum, lines[i]))
		} else {
			sb.WriteString(fmt.Sprintf("  %d | %s\n", lineNum, lines[i]))
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

// formatSearchResponse 将搜索结果格式化为合约定义的文本输出
func formatSearchResponse(resp *storage.SearchResponse) string {
	if resp.TotalCount == 0 {
		return fmt.Sprintf("未找到包含 '%s' 的内容。", resp.Query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("搜索 \"%s\" 的结果（显示 %d/%d 条）：\n",
		resp.Query, len(resp.Results), resp.TotalCount))

	for _, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("\n---\n📄 %s（第 %d 行）\n\n%s\n",
			r.FilePath, r.LineNumber, r.Context))
	}

	if resp.Truncated {
		sb.WriteString(fmt.Sprintf("\n还有 %d 条结果未显示。\n", resp.TotalCount-len(resp.Results)))
	}

	return sb.String()
}
