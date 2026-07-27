package conf

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/contract/log"
	"bit-labs.cn/owl/utils"
	"github.com/asaskevich/EventBus"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/viper"
)

var configErrorLinePattern = regexp.MustCompile(`(?i)\bline\s+(\d+)\b`)

type Configure struct {
	confDir        string
	cfgFileNameMap map[string]map[string]any // fileName => map[string]any
	app            foundation.Application
	eventBus       EventBus.Bus
	l              log.Logger
}

func NewConfigure(app foundation.Application, bus EventBus.Bus) *Configure {

	confDir := app.GetConfigPath()

	utils.PrintLnYellow("配置文件目录: ", confDir)

	manager := Configure{
		confDir:        confDir,
		cfgFileNameMap: make(map[string]map[string]any),
		eventBus:       bus,
		app:            app,
	}

	// 加载 .env 文件
	manager.loadEnvFiles()

	err := filepath.Walk(confDir, func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		cfgMap := make(map[string]any)
		ext := strings.Replace(filepath.Ext(info.Name()), ".", "", -1)
		name := strings.Replace(info.Name(), "."+ext, "", -1)
		absPath, v := manager.load(name, ext, &cfgMap)
		cfgMap["abs-path"] = absPath
		cfgMap["vip"] = v
		manager.cfgFileNameMap[name] = cfgMap
		return nil
	})
	if err != nil {
		panic(err)
	}
	return &manager
}

// loadEnvFiles 加载环境变量文件
func (i *Configure) loadEnvFiles() {
	// 获取应用根目录
	basePath := i.app.GetBasePath()

	// 尝试加载多个可能的 .env 文件，按优先级顺序
	envFiles := []string{
		filepath.Join(basePath, ".env.local"),               // 本地环境（最高优先级）
		filepath.Join(basePath, ".env."+i.getEnvironment()), // 环境特定文件
		filepath.Join(basePath, ".env"),                     // 默认环境文件
	}

	for _, envFile := range envFiles {
		if _, err := os.Stat(envFile); err == nil {
			if err := godotenv.Load(envFile); err != nil {
				fmt.Printf("Warning: Failed to load env file %s: %v\n", envFile, err)
			} else {
				fmt.Printf("Loaded environment file: %s\n", envFile)
			}
		}
	}
}

// getEnvironment 获取当前环境
func (i *Configure) getEnvironment() string {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENVIRONMENT")
	}
	if env == "" {
		env = "development"
	}
	return env
}

func (i *Configure) GetConfig(key string, v any) error {
	var getter jsoniter.Any

	marshal, err := jsoniter.Marshal(i.cfgFileNameMap)
	if err != nil {
		return err
	}

	pathArr := strings.Split(key, ".")

	for _, path := range pathArr {
		if getter != nil {
			getter = jsoniter.Get([]byte(getter.ToString()), path)
		} else {
			getter = jsoniter.Get(marshal, path)
		}
	}

	if getter != nil {
		err = jsoniter.UnmarshalFromString(getter.ToString(), v)
	}
	return err
}

func (i *Configure) SaveConfig(fileName string, key string, value any) {
	cfg, ok := i.cfgFileNameMap[fileName]
	if ok {
		v := cfg["vip"].(*viper.Viper)
		v.Set(key, value)
		err := v.WriteConfig()
		if err != nil {
			fmt.Println("保存配置失败")
		}
	}
}

// load 读取文件中的配置
func (i *Configure) load(fileName, cfgType string, c any) (string, *viper.Viper) {

	confFilePath := filepath.Join(i.confDir, fileName+"."+cfgType)

	v := viper.New()
	v.SetConfigType(cfgType)
	v.SetConfigFile(confFilePath)

	// 启用环境变量支持
	v.AutomaticEnv()

	// 设置环境变量前缀，支持按模块分组
	envPrefix := strings.ToUpper(fileName)
	v.SetEnvPrefix(envPrefix)

	// 设置环境变量键名替换规则
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	//i.l.Info("配置文件: ", confFilePath)
	cfg, err := os.ReadFile(confFilePath)
	if err != nil {
		panic(formatConfigFileError("读取", confFilePath, err))
	}
	if err = v.ReadConfig(bytes.NewReader(cfg)); err != nil {
		panic(formatConfigFileError("解析", confFilePath, err))
	}

	// 转换为结构体
	if err := v.Unmarshal(&c); err != nil {
		panic(formatConfigFileError("转换", confFilePath, err))
	}

	// Watch for changes in the config file
	v.WatchConfig()
	// Register a callback function to handle the changes
	v.OnConfigChange(func(e fsnotify.Event) {
		i.eventBus.Publish(ConfigChangeEvent, e)
	})
	return confFilePath, v
}

func formatConfigFileError(action, path string, err error) error {
	location := ""
	if matches := configErrorLinePattern.FindStringSubmatch(err.Error()); len(matches) == 2 {
		location = fmt.Sprintf("（第 %s 行）", matches[1])
	}

	return fmt.Errorf("%s配置文件失败：%s%s：%w", action, path, location, err)
}
