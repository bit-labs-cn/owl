package conf

import (
	"os"
	"path/filepath"
	"strings"

	"bit-labs.cn/owl/utils"
	"gopkg.in/yaml.v3"
)

// ValidateConfigKeys compares keys in the embedded default YAML with the on-disk config file.
// For any key present in embedded but missing on disk, a yellow warning is printed.
// Missing keys are reported at the shallowest level only (e.g. report "server.tls" not all its children).
func ValidateConfigKeys(providerName, fileName, embeddedContent, confDir string) {
	embeddedMap := make(map[string]any)
	if err := yaml.Unmarshal([]byte(embeddedContent), &embeddedMap); err != nil {
		return
	}

	diskPath := filepath.Join(confDir, fileName)
	diskBytes, err := os.ReadFile(diskPath)
	if err != nil {
		return
	}
	diskMap := make(map[string]any)
	if err := yaml.Unmarshal(diskBytes, &diskMap); err != nil {
		return
	}

	embeddedPaths := collectKeyPaths(embeddedMap, "")
	diskPaths := collectKeyPaths(diskMap, "")

	diskSet := make(map[string]bool)
	for _, p := range diskPaths {
		diskSet[p] = true
	}

	var missing []string
	for _, p := range embeddedPaths {
		if !diskSet[p] {
			missing = append(missing, p)
		}
	}

	// Report only shallowest missing paths (no path is a prefix of another reported path)
	reported := filterDeepestOnly(missing)
	for _, key := range reported {
		utils.PrintLnYellow("[配置检查] ", fileName, ": 缺少配置项 \"", key, "\" (来自 ", providerName, ")")
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
