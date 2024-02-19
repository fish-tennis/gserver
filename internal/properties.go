package internal

// 动态属性接口
type Properties interface {
	GetProperty(name string) interface{}
	GetPropertyString(name string) string
}

// 动态属性
type BaseProperties struct {
	Properties map[string]interface{} `json:"Properties"` // 动态属性
}

func (this *BaseProperties) GetProperty(name string) interface{} {
	if len(this.Properties) == 0 {
		return nil
	}
	return this.Properties[name]
}

func (this *BaseProperties) GetPropertyString(name string) string {
	value := this.GetProperty(name)
	if value == nil {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}

type PropertyInt32 interface {
	GetPropertyInt32(propertyName string) int32
}