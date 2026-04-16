package subutils

import (
	"bit-labs.cn/owl"
	"bit-labs.cn/owl/contract/foundation"
	"bit-labs.cn/owl/provider/router"
	subcmd "bit-labs.cn/owl/sub-utils/cmd"
	"github.com/spf13/cobra"
)

var _ owl.SubApp = (*SubAppUtils)(nil)

// SubAppUtils 命令行工具子应用（证书与授权等）。
type SubAppUtils struct {
	app foundation.Application
}

// Mount 将 sub-utils 子应用追加到 SubApp 列表。
func Mount(apps ...owl.SubApp) []owl.SubApp {
	return append(apps, &SubAppUtils{})
}

func (i *SubAppUtils) Name() string {
	return "sub-utils"
}

func (i *SubAppUtils) RegisterRouters() {}

func (i *SubAppUtils) ServiceProviders() []foundation.ServiceProvider {
	return []foundation.ServiceProvider{}
}

func (i *SubAppUtils) Binds() []any {
	return []any{}
}

func (i *SubAppUtils) Menu() []*router.Menu {
	return nil
}

func (i *SubAppUtils) Commands() []*cobra.Command {
	return []*cobra.Command{subcmd.NewUtilsCertCmd()}
}

func (i *SubAppUtils) Bootstrap() {}
