package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// ModelTypeName 返回 model 的稳定类型名（pkg.Name）。
func ModelTypeName(m any) string {
	if m == nil {
		return ""
	}
	t := reflect.TypeOf(m)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	return t.PkgPath() + "." + t.Name()
}

// ModelSchemaHash 对单个 GORM model 生成稳定 schema 指纹。
func ModelSchemaHash(m any) string {
	if m == nil {
		return ""
	}
	t := reflect.TypeOf(m)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return ""
	}
	fields := collectFieldDescriptors(t)
	sort.Strings(fields)

	var b strings.Builder
	b.WriteString(t.PkgPath() + "." + t.Name())
	b.WriteByte('\n')
	for _, f := range fields {
		b.WriteString(f)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

func collectFieldDescriptors(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		gormTag := f.Tag.Get("gorm")
		if gormTag == "-" {
			continue
		}
		if f.Anonymous {
			ft := f.Type
			for ft.Kind() == reflect.Ptr {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				out = append(out, collectFieldDescriptors(ft)...)
				continue
			}
		}
		out = append(out, fmt.Sprintf("%s|%s|%s", f.Name, f.Type.String(), gormTag))
	}
	return out
}
