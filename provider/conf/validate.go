package conf

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bit-labs.cn/owl/utils"
	"gopkg.in/yaml.v3"
)

// errNotBlockMapping 表示根节点不是可安全插入的块式映射。
var errNotBlockMapping = errors.New("yaml root is not a block mapping")

// ValidateConfigKeys 把嵌入的默认 YAML 与磁盘上的配置比较。
// 默认配置里有、磁盘上没有的键，会按默认配置中的相邻键插回原文件，不改已有的值、注释和其余行。
// 插不进去的键只打印告警。
func ValidateConfigKeys(providerName, fileName, embeddedContent, confDir string) {
	diskPath := filepath.Join(confDir, fileName)
	diskBytes, err := os.ReadFile(diskPath)
	if err != nil {
		return
	}

	out, added, missing, err := supplementYAML([]byte(embeddedContent), diskBytes)
	if err != nil {
		reportMissingMap(providerName, fileName, embeddedContent, diskBytes)
		return
	}
	if len(added) > 0 && !bytes.Equal(out, diskBytes) {
		perm := os.FileMode(0644)
		if info, statErr := os.Stat(diskPath); statErr == nil {
			perm = info.Mode().Perm()
		}
		if writeErr := os.WriteFile(diskPath, out, perm); writeErr != nil {
			utils.PrintLnYellow("[配置检查] ", fileName, ": 补充配置失败: ", writeErr.Error())
			for _, key := range added {
				reportMissing(providerName, fileName, key)
			}
			for _, key := range missing {
				reportMissing(providerName, fileName, key)
			}
			return
		}
		for _, key := range added {
			utils.PrintLnYellow("[配置检查] ", fileName, ": 已补充配置项 \"", key, "\" (来自 ", providerName, ")")
		}
	}
	for _, key := range missing {
		reportMissing(providerName, fileName, key)
	}
}

func reportMissing(providerName, fileName, key string) {
	utils.PrintLnYellow("[配置检查] ", fileName, ": 缺少配置项 \"", key, "\" (来自 ", providerName, ")")
}

// supplementYAML 把 embedded 中缺失的键补进 disk 的副本。
// added 是实际插入的键路径；missing 是没法安全插入的键路径。
func supplementYAML(embedded, disk []byte) (out []byte, added, missing []string, err error) {
	embDoc, err := parseYAMLDoc(embedded)
	if err != nil {
		return nil, nil, nil, err
	}
	diskDoc, err := parseYAMLDoc(disk)
	if err != nil {
		return nil, nil, nil, err
	}
	embRoot, ok := blockMappingRoot(embDoc)
	if !ok {
		return nil, nil, nil, errNotBlockMapping
	}
	diskRoot, ok := blockMappingRoot(diskDoc)
	if !ok {
		return nil, nil, nil, errNotBlockMapping
	}

	lines, _, _ := yamlLines(disk)
	indent := mappingIndent(diskDoc)
	var patches []yamlPatch
	order := 0
	mergeMissing(lines, diskRoot, embRoot, "", 1, indent, 0, &patches, &added, &missing, &order)
	if len(patches) == 0 {
		return disk, added, missing, nil
	}
	return applyPatches(disk, patches), added, missing, nil
}

func parseYAMLDoc(src []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if doc.Kind == 0 {
		return nil, errors.New("empty yaml document")
	}
	return &doc, nil
}

func blockMappingRoot(doc *yaml.Node) (*yaml.Node, bool) {
	if doc == nil {
		return nil, false
	}
	n := doc
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil, false
		}
		n = n.Content[0]
	}
	if n.Kind != yaml.MappingNode || n.Style&yaml.FlowStyle != 0 {
		return nil, false
	}
	return n, true
}

type yamlPatch struct {
	atLine int
	order  int
	lines  []string
}

// mergeMissing 按 src 的键顺序把 dest 里没有的键记成补丁。
// 新键紧挨着默认配置中它前面那个、且磁盘上已经存在的兄弟键；没有这样的兄弟时，插到后面那个已存在兄弟之前。
func mergeMissing(lines []string, dest, src *yaml.Node, prefix string, column, indent, parentLine int, patches *[]yamlPatch, added, missing *[]string, order *int) {
	if dest == nil || src == nil || dest.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return
	}
	if dest.Style&yaml.FlowStyle != 0 {
		*missing = append(*missing, missingInside(dest, src, prefix)...)
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		keyNode := src.Content[i]
		valNode := src.Content[i+1]
		if keyNode.Value == "<<" {
			continue
		}
		path := joinPath(prefix, keyNode.Value)
		di := indexOfKey(dest, keyNode.Value)
		if di < 0 {
			text, renderErr := renderKV(keyNode, valNode, keyColumn(dest, column), indent)
			if renderErr != nil {
				*missing = append(*missing, path)
				continue
			}
			parts := splitSnippet(text)
			if len(parts) == 0 {
				*missing = append(*missing, path)
				continue
			}
			*order++
			*patches = append(*patches, yamlPatch{
				atLine: insertionAt(lines, dest, src, i, parentLine),
				order:  *order,
				lines:  parts,
			})
			*added = append(*added, path)
			continue
		}
		destKey := dest.Content[di]
		destVal := dest.Content[di+1]
		if valNode.Kind == yaml.MappingNode && destVal.Kind == yaml.MappingNode && destVal.Style&yaml.FlowStyle == 0 {
			parentCol := destKey.Column
			if parentCol < 1 {
				parentCol = column
			}
			mergeMissing(lines, destVal, valNode, path, keyColumn(destVal, parentCol+indent), indent, destKey.Line, patches, added, missing, order)
			continue
		}
		if valNode.Kind == yaml.MappingNode {
			*missing = append(*missing, missingInside(destVal, valNode, path)...)
		}
	}
}

// missingInside 列出 dest 里放不进去的 src 键。父键已经存在、但值不是块式映射时用它。
func missingInside(dest, src *yaml.Node, prefix string) []string {
	if src == nil || src.Kind != yaml.MappingNode {
		if prefix == "" {
			return nil
		}
		return []string{prefix}
	}
	var out []string
	for i := 0; i+1 < len(src.Content); i += 2 {
		keyNode := src.Content[i]
		valNode := src.Content[i+1]
		if keyNode.Value == "<<" {
			continue
		}
		path := joinPath(prefix, keyNode.Value)
		di := -1
		if dest != nil && dest.Kind == yaml.MappingNode {
			di = indexOfKey(dest, keyNode.Value)
		}
		if di < 0 {
			out = append(out, path)
			continue
		}
		if valNode.Kind == yaml.MappingNode {
			out = append(out, missingInside(dest.Content[di+1], valNode, path)...)
		}
	}
	return out
}

func insertionAt(lines []string, dest, src *yaml.Node, srcKeyIndex, parentLine int) int {
	for j := srcKeyIndex - 2; j >= 0; j -= 2 {
		di := indexOfKey(dest, src.Content[j].Value)
		if di < 0 {
			continue
		}
		end := blockEndLine(lines, dest.Content[di], dest.Content[di+1])
		if end >= 1 {
			return end + 1
		}
	}
	for j := srcKeyIndex + 2; j+1 < len(src.Content); j += 2 {
		di := indexOfKey(dest, src.Content[j].Value)
		if di < 0 {
			continue
		}
		if line := dest.Content[di].Line; line >= 1 {
			return blockStartLine(lines, line)
		}
	}
	if len(dest.Content) >= 2 {
		last := len(dest.Content) - 2
		end := blockEndLine(lines, dest.Content[last], dest.Content[last+1])
		if end >= 1 {
			return end + 1
		}
	}
	if parentLine >= 1 {
		return parentLine + 1
	}
	return len(lines) + 1
}

func blockStartLine(lines []string, keyLine int) int {
	start := keyLine
	for start > 1 {
		prev := lines[start-2]
		if strings.TrimSpace(prev) == "" {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(prev), "#") {
			start--
			continue
		}
		break
	}
	return start
}

func blockEndLine(lines []string, key, val *yaml.Node) int {
	end := 0
	keyIndent := 0
	if key != nil {
		end = key.Line
		keyIndent = nodeIndent(lines, key)
	}
	if val != nil {
		if next := valueEndLine(lines, val, keyIndent); next > end {
			end = next
		}
	}
	end = consumeFoot(lines, end, key)
	end = consumeFoot(lines, end, val)
	return end
}

func valueEndLine(lines []string, n *yaml.Node, keyIndent int) int {
	if n == nil {
		return 0
	}
	end := n.Line
	switch n.Kind {
	case yaml.ScalarNode:
		if next := scalarEndLine(lines, n.Line, keyIndent); next > end {
			end = next
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			if next := blockEndLine(lines, n.Content[i], n.Content[i+1]); next > end {
				end = next
			}
		}
	case yaml.SequenceNode:
		for _, item := range n.Content {
			itemIndent := nodeIndent(lines, item)
			if item.Kind == yaml.ScalarNode || item.Kind == yaml.AliasNode {
				itemIndent = lineIndent(lines, item.Line)
			}
			if next := valueEndLine(lines, item, itemIndent); next > end {
				end = next
			}
		}
	case yaml.AliasNode:
		if n.Alias != nil {
			if next := valueEndLine(lines, n.Alias, keyIndent); next > end {
				end = next
			}
		}
	}
	return end
}

// scalarEndLine 把字面量块、折叠块以及缩进续行算进标量结尾。
func scalarEndLine(lines []string, startLine, keyIndent int) int {
	if startLine < 1 {
		return 0
	}
	end := startLine
	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			if !moreIndentedAhead(lines, i+1, keyIndent) {
				break
			}
			end = i + 1
			continue
		}
		if indentOf(line) > keyIndent {
			end = i + 1
			continue
		}
		break
	}
	return end
}

func moreIndentedAhead(lines []string, from, keyIndent int) bool {
	for i := from; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			continue
		}
		return indentOf(line) > keyIndent
	}
	return false
}

func consumeFoot(lines []string, end int, n *yaml.Node) int {
	if n == nil || end < 1 {
		return end
	}
	need := commentLineCount(n.FootComment)
	for i := 0; i < need; i++ {
		next := end + 1
		if next > len(lines) {
			break
		}
		if !strings.HasPrefix(strings.TrimSpace(lines[next-1]), "#") {
			break
		}
		end = next
	}
	return end
}

func commentLineCount(comment string) int {
	comment = strings.TrimRight(comment, "\n")
	if strings.TrimSpace(comment) == "" {
		return 0
	}
	return strings.Count(comment, "\n") + 1
}

func renderKV(key, val *yaml.Node, column, indent int) (string, error) {
	doc := &yaml.Node{
		Kind:    yaml.MappingNode,
		Content: []*yaml.Node{key, val},
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(indent)
	if err := enc.Encode(doc); err != nil {
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	raw := strings.TrimRight(strings.ReplaceAll(buf.String(), "\r\n", "\n"), "\n")
	if raw == "" {
		return "", errors.New("empty yaml snippet")
	}
	pad := ""
	if column > 1 {
		pad = strings.Repeat(" ", column-1)
	}
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		if line == "" {
			b.WriteByte('\n')
			continue
		}
		b.WriteString(pad)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String(), nil
}

func splitSnippet(text string) []string {
	text = strings.TrimRight(text, "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func applyPatches(content []byte, patches []yamlPatch) []byte {
	lines, nl, endsWithNL := yamlLines(content)
	sort.Slice(patches, func(i, j int) bool {
		if patches[i].atLine != patches[j].atLine {
			return patches[i].atLine > patches[j].atLine
		}
		return patches[i].order > patches[j].order
	})
	for _, patch := range patches {
		idx := patch.atLine - 1
		if idx < 0 {
			idx = 0
		}
		if idx > len(lines) {
			idx = len(lines)
		}
		next := make([]string, 0, len(lines)+len(patch.lines))
		next = append(next, lines[:idx]...)
		next = append(next, patch.lines...)
		next = append(next, lines[idx:]...)
		lines = next
	}
	out := strings.Join(lines, nl)
	if endsWithNL {
		out += nl
	}
	return []byte(out)
}

func yamlLines(content []byte) (lines []string, nl string, endsWithNL bool) {
	nl = "\n"
	if bytes.Contains(content, []byte("\r\n")) {
		nl = "\r\n"
	}
	text := strings.ReplaceAll(string(content), "\r\n", "\n")
	endsWithNL = text == "" || strings.HasSuffix(text, "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return nil, nl, endsWithNL
	}
	return strings.Split(text, "\n"), nl, endsWithNL
}

func mappingIndent(n *yaml.Node) int {
	if d, ok := findIndent(n); ok && d > 0 {
		return d
	}
	return 2
}

func findIndent(n *yaml.Node) (int, bool) {
	if n == nil {
		return 0, false
	}
	switch n.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range n.Content {
			if d, ok := findIndent(child); ok {
				return d, true
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(n.Content); i += 2 {
			key, val := n.Content[i], n.Content[i+1]
			if val.Kind == yaml.MappingNode && val.Style&yaml.FlowStyle == 0 {
				for j := 0; j+1 < len(val.Content); j += 2 {
					child := val.Content[j]
					if child.Column > key.Column {
						return child.Column - key.Column, true
					}
				}
			}
			if d, ok := findIndent(val); ok {
				return d, true
			}
		}
	}
	return 0, false
}

func keyColumn(mapping *yaml.Node, fallback int) int {
	if mapping != nil {
		for i := 0; i+1 < len(mapping.Content); i += 2 {
			if mapping.Content[i].Column > 0 {
				return mapping.Content[i].Column
			}
		}
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func indexOfKey(mapping *yaml.Node, name string) int {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return -1
	}
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == name {
			return i
		}
	}
	return -1
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func nodeIndent(lines []string, n *yaml.Node) int {
	if n == nil {
		return 0
	}
	if n.Column > 0 {
		return n.Column - 1
	}
	return lineIndent(lines, n.Line)
}

func lineIndent(lines []string, line int) int {
	if line < 1 || line > len(lines) {
		return 0
	}
	return indentOf(lines[line-1])
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' && r != '\t' {
			break
		}
		n++
	}
	return n
}

// reportMissingMap 在无法安全改写文件时，沿用原来的缺键告警。
func reportMissingMap(providerName, fileName, embeddedContent string, diskBytes []byte) {
	embeddedMap := make(map[string]any)
	if err := yaml.Unmarshal([]byte(embeddedContent), &embeddedMap); err != nil {
		return
	}
	diskMap := make(map[string]any)
	if err := yaml.Unmarshal(diskBytes, &diskMap); err != nil {
		return
	}

	diskSet := make(map[string]bool)
	for _, p := range collectKeyPaths(diskMap, "") {
		diskSet[p] = true
	}
	var missing []string
	for _, p := range collectKeyPaths(embeddedMap, "") {
		if !diskSet[p] {
			missing = append(missing, p)
		}
	}
	for _, key := range filterDeepestOnly(missing) {
		reportMissing(providerName, fileName, key)
	}
}

// collectKeyPaths recursively collects dot-separated key paths from a map (and nested maps).
// Values that are not maps (e.g. scalars, slices) are treated as leaves; the path to the key is included.
func collectKeyPaths(m map[string]any, prefix string) []string {
	var paths []string
	for k, v := range m {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		paths = append(paths, path)
		if nested := toMapStringAny(v); nested != nil {
			paths = append(paths, collectKeyPaths(nested, path)...)
		}
	}
	return paths
}

// toMapStringAny converts map[interface{}]interface{} (from yaml) or map[string]any to map[string]any.
func toMapStringAny(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	if m, ok := v.(map[interface{}]interface{}); ok {
		out := make(map[string]any, len(m))
		for k, val := range m {
			if s, ok := k.(string); ok {
				out[s] = val
			}
		}
		return out
	}
	return nil
}

// filterDeepestOnly keeps only paths that are not a strict prefix of any other path in the list.
// So we report the root of each missing subtree (e.g. "server.tls" instead of server.tls.*).
func filterDeepestOnly(paths []string) []string {
	var result []string
	for _, p := range paths {
		isPrefix := false
		for _, q := range paths {
			if p != q && strings.HasPrefix(q, p+".") {
				isPrefix = true
				break
			}
		}
		if !isPrefix {
			result = append(result, p)
		}
	}
	return result
}
