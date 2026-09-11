package db

import (
	"fmt"
	"reflect"
	"strings"

	"bit-labs.cn/owl/utils"
	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

// GetMenuModelsByIDs 获取菜单模型，gorm 多对多关联写数据

func GetModelsByIDs[T any](modelIDs []string) []T {
	var models []T
	for _, id := range modelIDs {
		id = strings.TrimSpace(id)
		if id == "" || id == "0" {
			continue
		}
		m := *new(T)
		value := reflect.ValueOf(&m).Elem()
		fieldType := value.FieldByName("ID").Type()
		switch fieldType.Kind() {
		case reflect.Uint:
			uid := cast.ToUint64(id)
			if uid == 0 {
				continue
			}
			value.FieldByName("ID").SetUint(uid)
		case reflect.String:
			value.FieldByName("ID").SetString(id)
		default:
			panic("unhandled default case")
		}

		models = append(models, m)
	}
	return models
}

// ReplaceJoinTable 只维护 many2many 中间表，不 Create/Update 关联实体。
// GORM 1.25+ 的 Association.Replace 会 upsert 关联对象；传入仅有 ID 的零值 struct 会插入空记录。
// 另外 Omit("Assoc.*") 后 Replace 不会删掉「相同关联 ID」的旧中间表行，中间表若无唯一约束就会 1→2→4 翻倍。
func ReplaceJoinTable(tx *gorm.DB, model any, association string, values any) error {
	values = uniqueAssocByID(values)
	if err := tx.Model(model).Association(association).Clear(); err != nil {
		return err
	}
	rv := reflect.ValueOf(values)
	if !rv.IsValid() || (rv.Kind() == reflect.Slice && rv.Len() == 0) {
		return nil
	}
	return tx.Omit(association+".*").Model(model).Association(association).Append(values)
}

func uniqueAssocByID(values any) any {
	rv := reflect.ValueOf(values)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return values
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Slice {
		return values
	}
	seen := map[string]struct{}{}
	out := reflect.MakeSlice(rv.Type(), 0, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		idField := reflect.Indirect(elem).FieldByName("ID")
		if !idField.IsValid() {
			out = reflect.Append(out, elem)
			continue
		}
		key := fmt.Sprint(idField.Interface())
		if key == "" || key == "0" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = reflect.Append(out, elem)
	}
	return out.Interface()
}

// AppendWhereFromStruct 根据结构体中的字段名，生成 where 条件
// NameLike = name like
// AgeGt = where age >
// AgeLt = where age <
func AppendWhereFromStruct(db *gorm.DB, s any) {
	result := make(map[string]interface{})
	v := reflect.ValueOf(s)
	if v.Equal(reflect.ValueOf(nil)) {
		return
	}

	// 兼容指针结构体和结构体
	if v.Kind() == reflect.Struct || v.Kind() == reflect.Ptr {
		if v.Kind() == reflect.Ptr && !v.IsNil() {
			v = v.Elem()
		}
	}

	getNonZeroFields(v, result)

	// 字段名可能叫 NameLike, AgeGt ...
	for field, value := range result {
		realField, operator := splitStringAtLastUnderscore(field)
		switch operator {
		case "gt":
			db.Where(fmt.Sprintf("%s > ?", realField), value)
		case "gte":
			db.Where(fmt.Sprintf("%s >= ?", realField), value)
		case "lt":
			db.Where(fmt.Sprintf("%s < ?", realField), value)
		case "lte":
			db.Where(fmt.Sprintf("%s < ?", realField), value)
		case "in":
			str, ok := value.(string)

			if ok {
				var ids []string
				// 处理字符串 "["1", "2", "3"]"
				if strings.Contains(str, "[") {
					_ = jsoniter.Unmarshal([]byte(str), &ids)
				} else {
					// 处理字符串 "1", "2", "3"
					ids = strings.Split(str, ",")
				}

				db.Where(fmt.Sprintf("%s in ?", realField), ids)
			} else {
				db.Where(fmt.Sprintf("%s in ?", realField), value)
			}

		case "notin":
			db.Where(fmt.Sprintf("%s not in ?", realField), value)
		case "like":
			db.Where(fmt.Sprintf("%s like ?", realField), "%"+cast.ToString(value)+"%")
		case "between":
			valueArr := strings.Split(cast.ToString(value), ",")
			if len(valueArr) == 2 {
				db.Where(fmt.Sprintf("%s >= ? and %s <= ?", realField, realField), valueArr[0], valueArr[1])
			} else {
				//logx.Error("between operator must be two value")
			}

		default:
			db.Where(fmt.Sprintf("%s = ?", field), value)
		}
	}
}

func splitStringAtLastUnderscore(s string) (string, string) {
	// 找到最后一个 '_' 的位置
	lastIndex := strings.LastIndex(s, "_")

	// 判断是否有 '_' 存在
	if lastIndex == -1 {
		return s, ""
	}

	// 分割字符串
	prefix := s[:lastIndex]
	suffix := s[lastIndex+1:]

	return prefix, suffix
}

// 定义一个函数来递归获取结构体的所有字段，并排除零值的字段
func getNonZeroFields(v reflect.Value, result map[string]interface{}) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// 过滤掉特定的字段
		if field.Name == "PageReq" {
			continue
		}

		// 检查字段是否为零值
		isZero := fieldValue.Interface() == reflect.Zero(fieldValue.Type()).Interface()
		if isZero {
			continue
		}

		// 如果字段是结构体类型，则递归调用
		if fieldValue.Kind() == reflect.Struct || fieldValue.Kind() == reflect.Ptr {
			if fieldValue.Kind() == reflect.Ptr && !fieldValue.IsNil() {
				fieldValue = fieldValue.Elem()
			}
			getNonZeroFields(fieldValue, result)
		} else {
			// 将非零值字段添加到结果中
			result[utils.Cc2Udl(field.Name)] = fieldValue.Interface()
		}
	}
}
