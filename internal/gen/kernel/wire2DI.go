package kernel

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Wire2DIFile 在 filePath 指向的位置扫描 DI 文件并写入注入内容。
// filePath 既可以是 DI 文件所在目录,也可以是 DI 文件本身(此时取其所在目录),
// 还可以是生成代码所在目录(此时向上逐级查找含 marker 的 DI 文件)。
//
// B7 并发安全声明: wireProcess 是 read-modify-write 操作，目前调用方（kun create / kun wire all）
// 均为串行逢历，无并发写问题。若未来引入并发，需对 DI 文件加文件锁。
func Wire2DIFile(filePath string, contentMap map[string]string) error {
	// 确定起始扫描目录:filePath 为目录则直接用,为文件则取其所在目录。
	dirName := filePath
	if info, err := os.Stat(dirName); err == nil && !info.IsDir() {
		dirName = filepath.Dir(dirName)
	}

	// 向上逐级查找含任一 marker 的 DI 文件,最多上探 4 级,覆盖
	// db/cache(生成目录的父级)与 hdl/svc/router(生成目录本身)两种 DI 文件布局。
	diFiles := findDIFiles(dirName, contentMap, 4)
	if len(diFiles) == 0 {
		return fmt.Errorf("the DI file does not exist near %s", filePath)
	}

	// 每个 DI 文件通常只含 contentMap 中的部分 marker(如 db 的 DI 文件不含 router 的 marker),
	// 因此先判断该文件是否真的含有此 marker,再注入 —— 避免把"本文件不该有的 marker"
	// 误判为注入失败。同时统计成功注入数,确保每个 marker 至少被处理一次。
	injected := make(map[string]bool, len(contentMap))
	for _, diFile := range diFiles {
		content, err := os.ReadFile(diFile)
		if err != nil {
			return fmt.Errorf("read DI file %s failed: %w", diFile, err)
		}
		for markerLine, appendContent := range contentMap {
			if !strings.Contains(string(content), markerLine) {
				continue
			}
			if err := wireProcess(diFile, markerLine, appendContent); err != nil {
				return fmt.Errorf("processing files %s Failed: %w", diFile, err)
			}
			injected[markerLine] = true
		}
	}

	// 有 marker 完全没落地 => DI 注入不完整,必须报错而不是静默成功,
	// 否则用户看到绿色成功提示但代码没接上,排查成本极高。
	var missing []string
	for markerLine := range contentMap {
		if !injected[markerLine] {
			missing = append(missing, markerLine)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("DI 注入不完整,以下 marker 未在任何 DI 文件中找到: %s\n", strings.Join(missing, " | ")))
		sb.WriteString("建议在相关 wire DI 文件中补齐对应 marker,或手动添加以下代码片段:\n")
		for _, m := range missing {
			sb.WriteString(fmt.Sprintf("  - Marker: %s\n    Code:\n%s\n", m, contentMap[m]))
		}
		return errors.New(strings.TrimRight(sb.String(), "\n"))
	}
	return nil
}

// findDIFiles 从 dir 开始向上逐级(最多 maxUp 级)扫描,返回所有包含 contentMap 中任一 marker 的文件。
func findDIFiles(dir string, contentMap map[string]string, maxUp int) []string {
	var diFiles []string
	for i := 0; i <= maxUp; i++ {
		fileInfos, err := os.ReadDir(dir)
		if err != nil {
			break
		}
		for _, fileInfo := range fileInfos {
			if fileInfo.IsDir() {
				continue
			}
			// P3: 先按文件名过滤(只读 .go 文件)，避免对非 Go 文件的内容搜索。
			// DI 文件必然是 .go 文件，提前过滤可显著减少 I/O。
			if filepath.Ext(fileInfo.Name()) != ".go" {
				continue
			}
			p := filepath.Join(dir, fileInfo.Name())
			content, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			for markerLine := range contentMap {
				if bytes.Contains(content, []byte(markerLine)) {
					diFiles = append(diFiles, p)
					break
				}
			}
		}
		if len(diFiles) > 0 {
			return diFiles
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // 已到文件系统根
		}
		dir = parent
	}
	return diFiles
}

// 写入内容到文件
func wireProcess(filePath, markerLine, appendContent string) error {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	text := string(content)

	// 探测主导行尾并全程复用。若按 "\n" 切分再用 "\n" 拼接,CRLF 文件的每行会残留 "\r",
	// 新插入的行却是纯 LF,产生混合行尾;Windows 上用编辑器保存过的文件必然踩中。
	newline := "\n"
	if strings.Contains(text, "\r\n") {
		newline = "\r\n"
	}

	// 幂等检查:按"压缩空白后"的形态比对,而不是精确字节比对。
	// gofmt 会重排缩进,精确比对在格式化之后就会失效,导致同一段内容被重复注入
	// (第二次运行 kun create 就把项目改坏)。
	if isAlreadyInjected(text, appendContent) {
		return nil
	}

	// 处理文件内容
	lines := strings.Split(text, "\n")
	var newContent []string

	markerFound := false
	for _, line := range lines {
		// 逐行剥离 "\r",统一在拼接时按 newline 写回,避免行尾混杂
		trimmedLine := strings.TrimSuffix(line, "\r")
		if strings.Contains(trimmedLine, markerLine) {
			newContent = append(newContent, appendContent)
			markerFound = true
		}
		newContent = append(newContent, trimmedLine)
	}

	// 找不到锚点说明目标文件被改过或路径不对。必须报错:
	// 静默返回 nil 会让 kun 打印成功却什么都没生成,用户毫无线索。
	if !markerFound {
		return fmt.Errorf("marker %q not found in %s: DI 注入失败(文件可能已被手工修改)", markerLine, filePath)
	}

	// 写回文件
	return os.WriteFile(filePath, []byte(strings.Join(newContent, newline)), 0644)
}

// isAlreadyInjected 判断 appendContent 是否已存在于 text 中。
// 按空白归一化后比对(每行 strings.Fields 再用单空格连接),使判断不受缩进、
// 行尾风格与 gofmt 重排的影响。
func isAlreadyInjected(text, appendContent string) bool {
	want := normalizeWhitespace(appendContent)
	if want == "" {
		return false
	}
	return strings.Contains(normalizeWhitespace(text), want)
}

// normalizeWhitespace 把任意连续空白(空格/制表/换行/回车)压缩成单个空格。
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
