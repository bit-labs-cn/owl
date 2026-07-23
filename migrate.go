package owl

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"bit-labs.cn/owl/internal/utils"
	"golang.org/x/sync/errgroup"
	"gorm.io/gorm"
)

const migrateHashFile = "migrate_hash.txt"

// runAutoMigrate 执行各 SubApp 的 BeforeMigrate →（按 model hash 门控 AutoMigrate）→ AfterMigrate。
// BeforeMigrate / AutoMigrate 串行；schema hash 与 AfterMigrate 在子应用间并行，AfterMigrate 全部完成（或任一失败）后再返回。
func (i *Application) runAutoMigrate(gdb *gorm.DB) error {
	totalStart := time.Now()

	stepStart := time.Now()
	for _, app := range i.subApps {
		appStart := time.Now()
		if err := app.BeforeMigrate(gdb); err != nil {
			return err
		}
		i.debugStartup(fmt.Sprintf("BeforeMigrate[%s]", app.Name()), appStart)
	}
	i.debugStartup("BeforeMigrate(all)", stepStart)

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
		for _, m := range models {
			name := utils.ModelTypeName(m)
			if name == "" {
				continue
			}
			if _, ok := typeToModel[name]; ok {
				continue
			}
			typeToModel[name] = m
			typeNames = append(typeNames, name)
		}
		sort.Strings(typeNames)

		// 并行计算各 model 的 schema hash（纯 CPU，按索引写回无竞争）
		stepStart = time.Now()
		hashes := make([]string, len(typeNames))
		var hashGroup errgroup.Group
		for idx, name := range typeNames {
			idx, name := idx, name
			m := typeToModel[name]
			hashGroup.Go(func() error {
				hashes[idx] = utils.ModelSchemaHash(m)
				return nil
			})
		}
		if err := hashGroup.Wait(); err != nil {
			return err
		}
		i.debugStartup(fmt.Sprintf("ModelSchemaHash(%d models)", len(typeNames)), stepStart)

		typeToHash := make(map[string]string, len(typeNames))
		changed := make([]any, 0)
		for idx, name := range typeNames {
			h := hashes[idx]
			typeToHash[name] = h
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
			stepStart = time.Now()
			if err := gdb.AutoMigrate(changed...); err != nil {
				return err
			}
			i.debugStartup(fmt.Sprintf("AutoMigrate(%d models)", len(changed)), stepStart)

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

	stepStart = time.Now()
	var afterGroup errgroup.Group
	for _, app := range i.subApps {
		app := app
		afterGroup.Go(func() error {
			appStart := time.Now()
			err := app.AfterMigrate(gdb)
			i.debugStartup(fmt.Sprintf("AfterMigrate[%s]", app.Name()), appStart)
			return err
		})
	}
	if err := afterGroup.Wait(); err != nil {
		return err
	}
	i.debugStartup("AfterMigrate(all)", stepStart)
	i.debugStartup("runAutoMigrate(total)", totalStart)
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
