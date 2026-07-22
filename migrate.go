package owl

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bit-labs.cn/owl/internal/utils"
	"gorm.io/gorm"
)

const migrateHashFile = "migrate_hash.txt"

// runAutoMigrate 执行各 SubApp 的 BeforeMigrate →（按 model hash 门控 AutoMigrate）→ AfterMigrate。
func (i *Application) runAutoMigrate(gdb *gorm.DB) error {
	for _, app := range i.subApps {
		if err := app.BeforeMigrate(gdb); err != nil {
			return err
		}
	}

	models := make([]any, 0)
	for _, app := range i.subApps {
		models = append(models, app.RegisterMigrate()...)
	}

	if len(models) > 0 {
		hashPath := filepath.Join(i.GetStoragePath(), migrateHashFile)
		prev := readMigrateHashes(hashPath)

		// 按类型去重，保留首次出现的实例
		typeNames := make([]string, 0, len(models))
		typeToModel := make(map[string]any, len(models))
		typeToHash := make(map[string]string, len(models))
		for _, m := range models {
			name := utils.ModelTypeName(m)
			if name == "" {
				continue
			}
			if _, ok := typeToModel[name]; ok {
				continue
			}
			typeToModel[name] = m
			typeToHash[name] = utils.ModelSchemaHash(m)
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)

		changed := make([]any, 0)
		for _, name := range typeNames {
			h := typeToHash[name]
			if prev[name] == h {
				continue
			}
			changed = append(changed, typeToModel[name])
		}

		if len(changed) == 0 {
			if i.l != nil {
				i.l.Info("all model schema hashes unchanged, skip AutoMigrate")
			}
		} else {
			if err := gdb.AutoMigrate(changed...); err != nil {
				return err
			}
			if err := os.MkdirAll(i.GetStoragePath(), 0755); err != nil {
				return err
			}
			if err := writeMigrateHashes(hashPath, typeToHash); err != nil {
				return err
			}
			if i.l != nil {
				i.l.Info(fmt.Sprintf("AutoMigrate %d changed model(s), schema hashes updated", len(changed)))
			}
		}
	}

	for _, app := range i.subApps {
		if err := app.AfterMigrate(gdb); err != nil {
			return err
		}
	}
	return nil
}

// readMigrateHashes 读取 name=hash 行；兼容旧版单行全局 hash（视为无效，全部重迁）。
func readMigrateHashes(path string) map[string]string {
	out := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		// 新格式：pkg.Type=hex
		if i := strings.IndexByte(line, '='); i > 0 {
			name := line[:i]
			hash := line[i+1:]
			if name != "" && hash != "" {
				out[name] = hash
			}
		}
	}
	return out
}

func writeMigrateHashes(path string, hashes map[string]string) error {
	names := make([]string, 0, len(hashes))
	for name := range hashes {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(hashes[name])
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0644)
}
