package common

import (
	"reflect"
	"testing"
)

func TestExtractCustomModels(t *testing.T) {
	for _, tt := range []struct {
		name   string
		config string
		want   []string
	}{
		{"空 config", "", nil},
		{"非法 JSON", "{not-json", nil},
		{"键缺失", `{"base_url":"https://x"}`, nil},
		{"类型不符（非数组）", `{"custom_models":"m1"}`, nil},
		{"基本提取", `{"custom_models":["m1","m2"]}`, []string{"m1", "m2"}},
		{"空白条目不算成员 + trim", `{"custom_models":[" m1 ","","  ",null,"m2"]}`, []string{"m1", "m2"}},
		{"非字符串元素跳过", `{"custom_models":["m1",42,{"a":1},"m2"]}`, []string{"m1", "m2"}},
		{"去重保留首现", `{"custom_models":["m1","m2","m1"]}`, []string{"m1", "m2"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractCustomModels(tt.config)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("ExtractCustomModels = %v, want %v", got, tt.want)
			}
		})
	}
}
