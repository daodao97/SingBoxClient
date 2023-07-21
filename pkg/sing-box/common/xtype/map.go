package xtype

import "github.com/spf13/cast"

type Map map[string]any

func (m Map) GetString(key string, defaultVal string) string {
	v, ok := m[key]
	if ok {
		return cast.ToString(v)
	}
	return defaultVal
}

func (m Map) GetInt(key string, defaultVal int) int {
	v, ok := m[key]
	if ok {
		return cast.ToInt(v)
	}
	return defaultVal
}

func (m Map) GetUInt16(key string, defaultVal uint16) uint16 {
	v, ok := m[key]
	if ok {
		return cast.ToUint16(v)
	}
	return defaultVal
}

func (m Map) GetBool(key string, defaultVal bool) bool {
	v, ok := m[key]
	if ok {
		return cast.ToBool(v)
	}
	return defaultVal
}
