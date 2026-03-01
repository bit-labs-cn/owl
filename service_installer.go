package owl

import (
	"embed"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"

	"bit-labs.cn/owl/utils"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
	"gopkg.in/guoliang1994/go-i18n.v2"
	"gopkg.in/guoliang1994/go-i18n.v2/driver"
)

type Installable interface {
	service.Interface
	GetBinName() string
	GetDisplayName() string
	GetDescription() string
	GetVersion() string
}

type ServiceCommandGetter struct {
	app     Installable
	service service.Service
	lang    *i18n.I18N
	rootCmd *cobra.Command
}

//go:embed lang
var langFs embed.FS

func NewServiceCommandGetter(app Installable) *cobra.Command {
	if err := IsValidBinaryName(app.GetBinName()); err != nil {
		panic("Invalid binary name: " + err.Error())
	}

	// Use Chinese as the default language
	// Modify the language using the Lang command
	embedDriver := driver.NewEmbedI18NImpl(langFs, "lang/")
	_, err := os.Stat("lang.conf")
	var l *i18n.I18N
	if err != nil {
		l = i18n.NewI18N(i18n.Chinese)
	} else {
		lang, _ := ioutil.ReadFile("lang.conf")
		l = i18n.NewI18N(string(lang))
	}
	l.AddLang(embedDriver)

	i := &ServiceCommandGetter{
		rootCmd: &cobra.Command{
			Use:   app.GetBinName(),
			Short: app.GetVersion(),
		},
		lang: l,
		app:  app,
	}
	i.rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// 初始化 service
	options := make(service.KeyValue)
	svcConfig := &service.Config{
		Name:        app.GetBinName(),
		DisplayName: app.GetDisplayName(),
		Description: app.GetDescription(),
		Option:      options,
	}

	if runtime.GOOS != "windows" {
		svcConfig.Dependencies = []string{
			"Requires=network.target",
			"After=network-online.target syslog.target"}
		svcConfig.UserName = "root"
	}

	svcConfig.Arguments = append(svcConfig.Arguments, "run")
	i.service, err = service.New(app, svcConfig)
	if err != nil {
		utils.PrintLnRed("Failed to create service: " + err.Error())
	}

	i.start()
	i.stop()
	i.restart()
	i.install()
	i.uninstall()
	i.status()
	i.run()
	i.Lang()

	return i.rootCmd
}
func (i *ServiceCommandGetter) install() {
	use := "install"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("install.short", i.app.GetBinName()),
		Long:  i.lang.T("install.long", i.app.GetBinName()),
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = i.service.Stop()
			_ = i.service.Uninstall()
			err := i.service.Install()
			if err != nil {
				return errors.New(i.lang.T("install.fail", i.app.GetBinName(), err.Error()))
			}
			fmt.Println(i.lang.T("install.success", i.app.GetBinName()))
			return nil
		},
	}
	i.rootCmd.AddCommand(c)
}

func (i *ServiceCommandGetter) uninstall() {
	use := "uninstall"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("uninstall.short", i.app.GetBinName()),
		Long:  i.lang.T("uninstall.long", i.app.GetBinName()),
		RunE:  i.control(use),
	}
	i.rootCmd.AddCommand(c)
}

func (i *ServiceCommandGetter) run() {
	use := "run"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("foreground.short", i.app.GetBinName()),
		Long:  i.lang.T("foreground.short", i.app.GetBinName()),
		RunE: func(cmd *cobra.Command, args []string) error {
			return i.service.Run()
		},
	}
	i.rootCmd.AddCommand(c)
}
func (i *ServiceCommandGetter) start() {
	use := "start"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("start.short", i.app.GetBinName()),
		Long:  i.lang.T("start.short", i.app.GetBinName()),
		RunE:  i.control(use),
	}
	i.rootCmd.AddCommand(c)
}

func (i *ServiceCommandGetter) stop() {
	use := "stop"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("stop.short", i.app.GetBinName()),
		Long:  i.lang.T("stop.long", i.app.GetBinName()),
		RunE:  i.control(use),
	}
	i.rootCmd.AddCommand(c)
}
func (i *ServiceCommandGetter) restart() {
	use := "restart"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("restart.short", i.app.GetBinName()),
		Long:  i.lang.T("restart.long", i.app.GetBinName()),
		RunE:  i.control(use),
	}
	i.rootCmd.AddCommand(c)
}
func (i *ServiceCommandGetter) status() {
	use := "status"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("status.short", i.app.GetBinName()),
		Long:  i.lang.T("status.short", i.app.GetBinName()),
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("")
		},
	}
	i.rootCmd.AddCommand(c)
}
func (i *ServiceCommandGetter) ver() {
	use := "version"
	var c = &cobra.Command{
		Use:   use,
		Short: i.lang.T("version.short", i.app.GetBinName()),
		Long:  i.lang.T("version.short", i.app.GetBinName()),
		Run: func(cmd *cobra.Command, args []string) {

		},
	}
	c.Flags()
	i.rootCmd.AddCommand(c)
}
func (i *ServiceCommandGetter) Lang() {
	use := "lang"
	var langCmd = &cobra.Command{
		Use:   use,
		Short: i.lang.T("lang.short", i.app.GetBinName()),
		Long:  i.lang.T("lang.short", i.app.GetBinName()),
		Run: func(cmd *cobra.Command, args []string) {
			lang := cmd.Flag("language").Value.String()
			err := ioutil.WriteFile("lang.conf", []byte(lang), 0777)
			if err != nil {
				return
			}
		},
	}
	var lang string
	langCmd.Flags().StringVarP(&lang, "language", "l", "zh", i.lang.T("lang.short"))
	i.rootCmd.AddCommand(langCmd)
}

// start stop restart
func (i *ServiceCommandGetter) control(command string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if service.Platform() == "unix-systemv" {
			terminal := exec.Command("/etc/init.d/"+i.app.GetBinName(), command)
			return terminal.Run()
		}
		err := service.Control(i.service, command)
		if err != nil {
			return errors.New(i.lang.T(command+".fail", i.app.GetBinName(), err.Error()))
		}
		return nil
	}
}

// IsValidBinaryName 检查二进制文件名是否合法
func IsValidBinaryName(binName string) error {
	// 检查是否为空
	if strings.TrimSpace(binName) == "" {
		return errors.New("binary name cannot be empty")
	}

	// 检查长度
	if len(binName) < 1 || len(binName) > 50 {
		return errors.New("binary name length must be between 1 and 50 characters")
	}

	// 使用正则表达式检查字符合法性
	// 允许字母、数字、下划线、连字符
	validPattern := regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)
	if !validPattern.MatchString(binName) {
		return errors.New("binary name can only contain letters, digits, underscores, and hyphens, and must start with a letter")
	}

	// 检查是否包含路径分隔符（这会是危险的）
	if strings.Contains(binName, "/") || strings.Contains(binName, "\\") {
		return errors.New("binary name cannot contain path separators")
	}

	return nil
}
