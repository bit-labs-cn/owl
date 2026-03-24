package permission

import (
	"bit-labs.cn/owl/contract/foundation"
	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var _ foundation.ServiceProvider = (*GuardProvider)(nil)

type GuardProvider struct {
	app foundation.Application
}

func (i *GuardProvider) Description() string {
	return "Casbin 用户权限引擎"
}

func (i *GuardProvider) Register() {
	i.app.Register(func(db *gorm.DB) *casbin.SyncedEnforcer {
		db = db.Session(&gorm.Session{Logger: db.Config.Logger.LogMode(logger.Error)})
		adapter, err := gormadapter.NewAdapterByDB(db)
		if err != nil {
			panic(err)
		}
		m, err := model.NewModelFromString(`
		[request_definition]
		r = sub, act
	
		[policy_definition]
		p = sub, act
	
		[role_definition]
		g = _, _
	
		[policy_effect]
		e = some(where (p.eft == allow))
	
		[matchers]
		m = g(r.sub, p.sub) && r.act == p.act
		`)
		if err != nil {
			return nil
		}
		enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
		if err != nil {
			panic(err)
		}
		return enforcer
	})
}
func (i *GuardProvider) Boot() {
}

func (i *GuardProvider) Conf() map[string]string {
	return nil
}
