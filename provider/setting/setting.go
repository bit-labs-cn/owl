// Package setting 让各子系统在代码里登记配置项。页面只能改已登记项的值。
package setting

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ValueType 是配置值的类型。
type ValueType string

const (
	// TypeString 文本。
	TypeString ValueType = "string"
	// TypeBool 布尔，值只接受 true 或 false。
	TypeBool ValueType = "bool"
	// TypeInt 整数。
	TypeInt ValueType = "int"
	// TypeImage 图片。值是 data:image/...;base64,...，解码后不能超过 ImageMaxBytes。
	TypeImage ValueType = "image"
)

// 单条文本配置的存储上限，避免超出数据库列宽。
const maxStoredRunes = 1024

// ImageMaxBytes 是图片配置解码后的最大字节数，200KB。
const ImageMaxBytes = 200 * 1024

// Item 是一项写死的配置。
type Item struct {
	Key     string
	Title   string
	Hint    string
	Type    ValueType
	Default string
	Public  bool
	Min     int // 整数下限，或文本最短长度。图片不使用。0 表示不限制。
	Max     int // 整数上限、文本最长长度，或图片解码后的最大字节数。0 表示不限制；图片仍不得超过 ImageMaxBytes。
}

// Accepts 报告 value 是否符合类型和取值范围。
func (it Item) Accepts(value string) error {
	switch it.Type {
	case TypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("%s只能是 true 或 false", it.Title)
		}
	case TypeInt:
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("%s必须是整数", it.Title)
		}
		if it.Min != 0 && n < it.Min {
			return fmt.Errorf("%s不能小于 %d", it.Title, it.Min)
		}
		if it.Max != 0 && n > it.Max {
			return fmt.Errorf("%s不能大于 %d", it.Title, it.Max)
		}
	case TypeString:
		n := utf8.RuneCountInString(value)
		limit := it.Max
		if limit == 0 || limit > maxStoredRunes {
			limit = maxStoredRunes
		}
		if it.Min != 0 && n < it.Min {
			return fmt.Errorf("%s不能短于 %d 个字符", it.Title, it.Min)
		}
		if n > limit {
			return fmt.Errorf("%s不能长于 %d 个字符", it.Title, limit)
		}
	case TypeImage:
		return acceptImage(it, value)
	default:
		return fmt.Errorf("%s的类型无效", it.Title)
	}
	return nil
}

func acceptImage(it Item, value string) error {
	if value == "" {
		return nil
	}
	prefix, payload, ok := strings.Cut(value, ",")
	magic, known := imageMagics[prefix]
	if !ok || !known || payload == "" {
		return fmt.Errorf("%s必须是 png、jpeg、gif 或 webp 的 base64", it.Title)
	}
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil || len(raw) == 0 {
		return fmt.Errorf("%s必须是 png、jpeg、gif 或 webp 的 base64", it.Title)
	}
	limit := ImageMaxBytes
	if it.Max != 0 && it.Max < limit {
		limit = it.Max
	}
	if len(raw) > limit {
		return fmt.Errorf("%s不能超过 %dKB", it.Title, (limit+1023)/1024)
	}
	if !magic(raw) {
		return fmt.Errorf("%s的内容和格式不一致", it.Title)
	}
	return nil
}

func pngMagic(raw []byte) bool {
	return len(raw) >= 8 && string(raw[:8]) == "\x89PNG\r\n\x1a\n"
}

func jpegMagic(raw []byte) bool {
	return len(raw) >= 3 && raw[0] == 0xff && raw[1] == 0xd8 && raw[2] == 0xff
}

func gifMagic(raw []byte) bool {
	return len(raw) >= 6 && (string(raw[:6]) == "GIF87a" || string(raw[:6]) == "GIF89a")
}

func webpMagic(raw []byte) bool {
	return len(raw) >= 12 && string(raw[:4]) == "RIFF" && string(raw[8:12]) == "WEBP"
}

var imageMagics = map[string]func([]byte) bool{
	"data:image/png;base64":  pngMagic,
	"data:image/jpeg;base64": jpegMagic,
	"data:image/gif;base64":  gifMagic,
	"data:image/webp;base64": webpMagic,
}

// Group 是子系统下的一级或二级分组。
// 一级可以只放配置项，也可以再放一组二级分组。二级只能放配置项。
// 同一组不能同时放配置项和子分组。Validate 只校验本组的配置项，不返回给前端。
type Group struct {
	Key      string
	Title    string
	Items    []Item
	Groups   []Group
	Validate func(values map[string]string) error
}

// Catalog 是一个子系统登记的全部分组。Title 是页面上的子系统名称。
type Catalog struct {
	Title  string
	Groups []Group
}

// Registrar 是可选能力。子系统实现它即可把自己的配置交给框架收集。
type Registrar interface {
	RegisterConfigs() Catalog
}

// Subsystem 是收集之后的一份配置，Name 来自 SubApp.Name()。
type Subsystem struct {
	Name   string
	Title  string
	Groups []Group
}
