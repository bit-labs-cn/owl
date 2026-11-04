package setting

import (
	"bytes"
	"encoding/base64"
	"strconv"
	"testing"
)

func TestRegistryAddAndLookup(t *testing.T) {
	reg := NewRegistry()
	err := reg.Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key:   "site",
			Title: "站点",
			Items: []Item{{
				Key:     "siteName",
				Title:   "站点名称",
				Type:    TypeString,
				Default: "",
				Max:     64,
				Public:  true,
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	item, ok := reg.Item("admin", "siteName")
	if !ok || item.Title != "站点名称" {
		t.Fatalf("item = %+v ok=%v", item, ok)
	}
	if _, ok := reg.Item("admin", "missing"); ok {
		t.Fatal("missing key should be absent")
	}
}

func TestRegistryRejectsDuplicateKey(t *testing.T) {
	reg := NewRegistry()
	err := reg.Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{
			{Key: "a", Title: "甲", Items: []Item{{Key: "same", Title: "一", Type: TypeBool, Default: "true"}}},
			{Key: "b", Title: "乙", Items: []Item{{Key: "same", Title: "二", Type: TypeBool, Default: "false"}}},
		},
	})
	if err == nil {
		t.Fatal("expected duplicate key error")
	}
}

func TestItemAcceptsBounds(t *testing.T) {
	item := Item{Title: "密码最短", Type: TypeInt, Min: 6, Max: 32, Default: "6"}
	if err := item.Accepts("5"); err == nil {
		t.Fatal("expected below min")
	}
	if err := item.Accepts("6"); err != nil {
		t.Fatal(err)
	}
	text := Item{Title: "站点名称", Type: TypeString, Max: 2, Default: "好"}
	if err := text.Accepts("好的呀"); err == nil {
		t.Fatal("expected over max")
	}
	open := Item{Title: "说明", Type: TypeString, Default: ""}
	if err := open.Accepts(""); err != nil {
		t.Fatal(err)
	}
}

func TestRegistryChecksGroupDefault(t *testing.T) {
	reg := NewRegistry()
	err := reg.Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key:   "account",
			Title: "账号",
			Items: []Item{
				{Key: "passwordMin", Title: "密码最短", Type: TypeInt, Default: "10", Min: 6, Max: 32},
				{Key: "passwordMax", Title: "密码最长", Type: TypeInt, Default: "8", Min: 8, Max: 64},
			},
			Validate: func(values map[string]string) error {
				min, _ := strconv.Atoi(values["passwordMin"])
				max, _ := strconv.Atoi(values["passwordMax"])
				if max < min {
					return errTest
				}
				return nil
			},
		}},
	})
	if err == nil {
		t.Fatal("expected group default validation error")
	}
}

func TestItemAcceptsImage(t *testing.T) {
	const png = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	item := Item{Title: "站点标志", Type: TypeImage, Max: ImageMaxBytes}
	if err := item.Accepts(""); err != nil {
		t.Fatal(err)
	}
	if err := item.Accepts(png); err != nil {
		t.Fatal(err)
	}
	if err := item.Accepts("https://example.com/a.png"); err == nil {
		t.Fatal("expected url to be rejected")
	}
	jpeg := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0x00})
	if err := item.Accepts(jpeg); err != nil {
		t.Fatal(err)
	}
	if err := item.Accepts("data:image/jpeg;base64," + base64.StdEncoding.EncodeToString([]byte("\x89PNG\r\n\x1a\nxxxx"))); err == nil {
		t.Fatal("expected mismatched content")
	}
	raw := bytes.Repeat([]byte{0xff}, ImageMaxBytes+1)
	over := "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(raw)
	if err := item.Accepts(over); err == nil {
		t.Fatal("expected over 200KB")
	}

	reg := NewRegistry()
	err := reg.Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key: "site", Title: "站点",
			Items: []Item{{Key: "logo", Title: "标志", Type: TypeImage, Max: ImageMaxBytes + 1}},
		}},
	})
	if err == nil {
		t.Fatal("expected image limit above 200KB to be rejected")
	}
}

func TestRegistryNestedGroup(t *testing.T) {
	reg := NewRegistry()
	err := reg.Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key: "site", Title: "站点",
			Groups: []Group{{
				Key: "brand", Title: "外观",
				Items: []Item{{Key: "siteName", Title: "站点名称", Type: TypeString, Default: ""}},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reg.Item("admin", "siteName"); !ok {
		t.Fatal("missing nested item")
	}
	group, ok := reg.GroupOf("admin", "siteName")
	if !ok || group.Title != "外观" {
		t.Fatalf("group = %+v ok=%v", group.Title, ok)
	}

	err = NewRegistry().Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key: "site", Title: "站点",
			Groups: []Group{{
				Key: "brand", Title: "外观",
				Groups: []Group{{
					Key: "logo", Title: "标志",
					Items: []Item{{Key: "siteLogo", Title: "站点标志", Type: TypeString, Default: ""}},
				}},
			}},
		}},
	})
	if err == nil {
		t.Fatal("expected a third group level to be rejected")
	}

	err = NewRegistry().Add("admin", Catalog{
		Title: "系统管理",
		Groups: []Group{{
			Key: "site", Title: "站点",
			Items: []Item{{Key: "siteName", Title: "站点名称", Type: TypeString, Default: ""}},
			Groups: []Group{{
				Key: "brand", Title: "外观",
				Items: []Item{{Key: "siteLogo", Title: "站点标志", Type: TypeString, Default: ""}},
			}},
		}},
	})
	if err == nil {
		t.Fatal("expected items and groups together to be rejected")
	}
}

var errTest = errString("最长不得短于最短")

type errString string

func (e errString) Error() string { return string(e) }
