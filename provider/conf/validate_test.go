package conf

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupplementOAuth2ScopesKeepsExistingValues(t *testing.T) {
	embedded := `
gitee:
  enable: false
  name: "Gitee"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/gitee/callback"
  scopes: ""

github:
  enable: false
  name: "GitHub"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/github/callback"
  scopes: ""

google:
  enable: false
  name: "Google"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/google/callback"
  scopes: ""

owl:
  enable: false
  name: "统一身份认证"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/owl/callback"
  scopes: "openid profile email phone"
  user-info-url: "http://127.0.0.1:8080/api/v1/oauth/userinfo"
  endpoint:
    auth-url: "http://127.0.0.1:8080/api/v1/oauth/authorize"
    token-url: "http://127.0.0.1:8080/api/v1/oauth/token"
    auth-style: in-params
`
	disk := `
gitee:
  enable: false
  name: "Gitee"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/gitee/callback"

github:
  enable: true
  name: "My GitHub"
  client-id: "id"
  client-secret: "secret"
  redirect-url: "https://example/oauth2/github/callback"

google:
  enable: false
  name: "Google"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/google/callback"

owl:
  enable: true
  name: "比特实验室"
  client-id: "cli"
  client-secret: "sec"
  redirect-url: "http://192.168.0.17:8080/oauth2/owl/callback"
  scopes: "openid profile"

  user-info-url: "http://192.168.0.17:8081/api/v1/oauth/userinfo"
  endpoint:
    auth-url: "http://192.168.0.17:8081/api/v1/oauth/authorize"
    token-url: "http://192.168.0.17:8081/api/v1/oauth/token"
    auth-style: in-params
`
	want := `
gitee:
  enable: false
  name: "Gitee"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/gitee/callback"
  scopes: ""

github:
  enable: true
  name: "My GitHub"
  client-id: "id"
  client-secret: "secret"
  redirect-url: "https://example/oauth2/github/callback"
  scopes: ""

google:
  enable: false
  name: "Google"
  client-id: ""
  client-secret: ""
  redirect-url: "https://example/oauth2/google/callback"
  scopes: ""

owl:
  enable: true
  name: "比特实验室"
  client-id: "cli"
  client-secret: "sec"
  redirect-url: "http://192.168.0.17:8080/oauth2/owl/callback"
  scopes: "openid profile"

  user-info-url: "http://192.168.0.17:8081/api/v1/oauth/userinfo"
  endpoint:
    auth-url: "http://192.168.0.17:8081/api/v1/oauth/authorize"
    token-url: "http://192.168.0.17:8081/api/v1/oauth/token"
    auth-style: in-params
`
	got, added, missing := mustSupplement(t, embedded, disk)
	if string(got) != trimLeadingNL(want) {
		t.Fatalf("补充结果不符\n got:\n%s\nwant:\n%s", got, trimLeadingNL(want))
	}
	wantAdded := []string{"gitee.scopes", "github.scopes", "google.scopes"}
	if strings.Join(added, ",") != strings.Join(wantAdded, ",") {
		t.Fatalf("added = %v; want %v", added, wantAdded)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}

	again, added, missing := mustSupplement(t, embedded, string(got))
	if !bytes.Equal(again, got) || len(added) != 0 || len(missing) != 0 {
		t.Fatalf("再次补充仍有变化 added=%v missing=%v\n%s", added, missing, again)
	}
}

func TestSupplementInsertsBetweenCommentedKeys(t *testing.T) {
	embedded := `
# 驱动 mysql pgsql sqlite
driver: mysql
# 端口
port: 5432
# 数据库名
database: app
`
	disk := `
# 驱动 mysql pgsql sqlite
driver: mysql
# 数据库名
database: kept
`
	want := `
# 驱动 mysql pgsql sqlite
driver: mysql
# 端口
port: 5432
# 数据库名
database: kept
`
	got, added, missing := mustSupplement(t, embedded, disk)
	if string(got) != trimLeadingNL(want) {
		t.Fatalf("补充结果不符\n got:\n%s\nwant:\n%s", got, trimLeadingNL(want))
	}
	if strings.Join(added, ",") != "port" || len(missing) != 0 {
		t.Fatalf("added=%v missing=%v", added, missing)
	}
}

func TestSupplementKeepsBlockScalarAndSequenceIntact(t *testing.T) {
	embedded := `
desc: |
  hello
  world
hooks:
  - a
  - b
port: 1
name: app
`
	disk := `
desc: |
  hello
  world
hooks:
  - a
  - b
name: kept
`
	want := `
desc: |
  hello
  world
hooks:
  - a
  - b
port: 1
name: kept
`
	got, added, _ := mustSupplement(t, embedded, disk)
	if string(got) != trimLeadingNL(want) {
		t.Fatalf("补充结果不符\n got:\n%s\nwant:\n%s", got, trimLeadingNL(want))
	}
	if strings.Join(added, ",") != "port" {
		t.Fatalf("added = %v", added)
	}
}

func TestSupplementNestedKeyAndDoesNotOverwrite(t *testing.T) {
	embedded := `
owl:
  enable: false
  scopes: "openid"
  endpoint:
    auth-url: a
    token-url: b
    device-auth-url: d
    auth-style: in-params
`
	disk := `
owl:
  enable: true
  endpoint:
    auth-url: kept-auth
    token-url: kept-token
    auth-style: in-header
`
	want := `
owl:
  enable: true
  scopes: "openid"
  endpoint:
    auth-url: kept-auth
    token-url: kept-token
    device-auth-url: d
    auth-style: in-header
`
	got, added, missing := mustSupplement(t, embedded, disk)
	if string(got) != trimLeadingNL(want) {
		t.Fatalf("补充结果不符\n got:\n%s\nwant:\n%s", got, trimLeadingNL(want))
	}
	if strings.Join(added, ",") != "owl.scopes,owl.endpoint.device-auth-url" {
		t.Fatalf("added = %v", added)
	}
	if len(missing) != 0 {
		t.Fatalf("missing = %v", missing)
	}
}

func TestSupplementPreservesCRLF(t *testing.T) {
	embedded := "a: 1\nb: 2\n"
	disk := "a: 9\r\n"
	got, added, missing := mustSupplement(t, embedded, disk)
	if string(got) != "a: 9\r\nb: 2\r\n" {
		t.Fatalf("got %q added=%v missing=%v", got, added, missing)
	}
}

func TestSupplementFlowMappingWarnsWithoutRewrite(t *testing.T) {
	embedded := `
server:
  port: 80
  host: localhost
`
	disk := "server: {port: 80}\n"
	got, added, missing := mustSupplement(t, embedded, disk)
	if string(got) != disk {
		t.Fatalf("流式映射被改写: %q", got)
	}
	if len(added) != 0 {
		t.Fatalf("added = %v", added)
	}
	if strings.Join(missing, ",") != "server.host" {
		t.Fatalf("missing = %v", missing)
	}
}

func TestSupplementNewSectionBeforeFollowingKey(t *testing.T) {
	embedded := `
a: 1
b:
  d: 2
c: 3
`
	disk := `
# 自定义
custom: 9
a: 1
c: 3
`
	want := `
# 自定义
custom: 9
a: 1
b:
  d: 2
c: 3
`
	got, added, _ := mustSupplement(t, embedded, disk)
	if string(got) != trimLeadingNL(want) {
		t.Fatalf("补充结果不符\n got:\n%s\nwant:\n%s", got, trimLeadingNL(want))
	}
	if strings.Join(added, ",") != "b" {
		t.Fatalf("added = %v", added)
	}
}

func TestValidateConfigKeysWritesAndIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oauth2.yaml")
	if err := os.WriteFile(path, []byte("a: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	ValidateConfigKeys("演示", "oauth2.yaml", "a: 1\nb: 2\n", dir)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "a: 1\nb: 2\n" {
		t.Fatalf("got %q", got)
	}
	ValidateConfigKeys("演示", "oauth2.yaml", "a: 1\nb: 2\n", dir)
	again, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, got) {
		t.Fatalf("第二次启动改写了文件: %q", again)
	}
}

func TestValidateConfigKeysDoesNotCreateOrClobber(t *testing.T) {
	dir := t.TempDir()
	ValidateConfigKeys("演示", "missing.yaml", "a: 1\n", dir)
	if _, err := os.Stat(filepath.Join(dir, "missing.yaml")); !os.IsNotExist(err) {
		t.Fatal("不存在的配置文件被创建了")
	}

	path := filepath.Join(dir, "bad.yaml")
	original := []byte(":\n  - [")
	if err := os.WriteFile(path, original, 0644); err != nil {
		t.Fatal(err)
	}
	ValidateConfigKeys("演示", "bad.yaml", "a: 1\n", dir)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("非法 YAML 被改写: %q", got)
	}
}

func mustSupplement(t *testing.T, embedded, disk string) ([]byte, []string, []string) {
	t.Helper()
	got, added, missing, err := supplementYAML([]byte(trimLeadingNL(embedded)), []byte(trimLeadingNL(disk)))
	if err != nil {
		t.Fatalf("supplementYAML: %v", err)
	}
	return got, added, missing
}

func trimLeadingNL(s string) string {
	return strings.TrimPrefix(s, "\n")
}
