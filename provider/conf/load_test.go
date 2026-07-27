package conf

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatConfigFileErrorIncludesPathAndLine(t *testing.T) {
	err := formatConfigFileError(
		"解析",
		`D:\project\app\conf\database.yaml`,
		errors.New("While parsing config: yaml: line 2: did not find expected key"),
	)

	for _, want := range []string{
		`解析配置文件失败：D:\project\app\conf\database.yaml`,
		"第 2 行",
		"did not find expected key",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("错误信息 %q 未包含 %q", err, want)
		}
	}
}
