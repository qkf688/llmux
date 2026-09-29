package testutil

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// 冒烟覆盖门本身：键集合逐字相等（缺键 / 多键都红）、null 语义（Nilable）、
// 类型断言与重复表键。门自己坏了就没人守形状，故门必须有最小回归。
func TestAPIResponseKeysErr(t *testing.T) {
	want := []APIResponseKey{
		{Key: "id", WantSetType: gjson.Number},
		{Key: "flag", WantSetType: gjson.True, Nilable: true},
	}
	body := `{"id":1,"flag":true}`

	cases := []struct {
		name    string
		body    string
		path    string
		want    []APIResponseKey
		wantErr string // 空串 = 期望通过
	}{
		{name: "逐字相等通过", body: body, want: want},
		{name: "根外路径", body: `{"data":{"id":1,"flag":null}}`, path: "data", want: want},
		{name: "缺键红", body: `{"id":1}`, want: want, wantErr: "缺键: [flag]"},
		{name: "多键红", body: `{"id":1,"flag":true,"extra":1}`, want: want, wantErr: "多键: [extra]"},
		{name: "改名红", body: `{"ID":1,"flag":true}`, want: want, wantErr: "缺键: [id]"},
		{name: "nilable 置 null 通过", body: `{"id":1,"flag":null}`, want: want},
		{name: "非 nilable 置 null 红", body: `{"id":null,"flag":true}`, want: want, wantErr: "Nilable"},
		{name: "类型漂移红", body: `{"id":"1","flag":true}`, want: want, wantErr: "type = String, want Number"},
		{name: "表键重复红", body: body, want: []APIResponseKey{{Key: "id"}, {Key: "id"}}, wantErr: "重复"},
		{name: "路径不是对象红", body: `{"data":1}`, path: "data", want: want, wantErr: "不是对象"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := apiResponseKeysErr(tc.body, tc.path, tc.want)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want 包含 %q", err, tc.wantErr)
			}
		})
	}
}
