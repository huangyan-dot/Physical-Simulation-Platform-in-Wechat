package model

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON 是可存入 MySQL JSON 列、又能在 HTTP 响应里原样输出为 JSON 对象/数组的类型。
// 空值落库为 SQL NULL，序列化为 "null"。用于 experiments.config / experiments.target 等。
type JSON json.RawMessage

// Value 实现 driver.Valuer：落库
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

// Scan 实现 sql.Scanner：读库（MySQL JSON 列返回 []byte）
func (j *JSON) Scan(src interface{}) error {
	if src == nil {
		*j = JSON{}
		return nil
	}
	switch s := src.(type) {
	case []byte:
		*j = append(JSON{}, s...)
		return nil
	case string:
		*j = append(JSON{}, s...)
		return nil
	}
	return errors.New("model.JSON: unsupported scan source")
}

// MarshalJSON 原样输出（不做二次转义）
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

// UnmarshalJSON 原样保存
func (j *JSON) UnmarshalJSON(b []byte) error {
	if b == nil {
		*j = JSON{}
		return nil
	}
	*j = append(JSON{}, b...)
	return nil
}
