package testutil

import (
	"strings"
	"testing"

	"github.com/tidwall/gjson"
)

// 冒烟覆盖门本身：键集合逐字相等（缺键 / 多键都红）、null 语义（Nilable）、
// 三态方向（有值态禁 null / 空值态禁有值）、类型断言与重复表键。
// 门自己坏了就没人守形状，故门必须有最小回归。
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
		state   KeyState // 零值 = KeyStateAny
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

		// 三态方向：同一张表要按两种态各断言一次，两个方向都必须有区分度。
		{name: "有值态通过", body: body, state: KeyStateSet, want: want},
		{name: "有值态 nilable 置 null 红", body: `{"id":1,"flag":null}`, state: KeyStateSet, want: want, wantErr: "有值态"},
		{name: "空值态 nilable 置 null 通过", body: `{"id":1,"flag":null}`, state: KeyStateUnset, want: want},
		{name: "空值态 nilable 有值红", body: body, state: KeyStateUnset, want: want, wantErr: "空值态"},
		// 空值态只放过 Nilable 键的 null，非 Nilable 键的类型仍须匹配
		// （否则空值态会顺带变成「类型也不查」的后门）。
		{name: "空值态非 nilable 类型漂移仍红", body: `{"id":"1","flag":null}`, state: KeyStateUnset, want: want,
			wantErr: "type = String, want Number"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := apiResponseKeysErrInState(tc.body, tc.path, tc.want, tc.state)
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

// TestAssertAPIResponseKeysInState_AnyMatchesLegacy 锁「默认态零行为变化」：
// KeyStateAny 是既有调用方（只断键与类型、不管三态方向）的语义，
// 加方向校验不得改变它们的结论。
func TestAssertAPIResponseKeysInState_AnyMatchesLegacy(t *testing.T) {
	want := []APIResponseKey{
		{Key: "id", WantSetType: gjson.Number},
		{Key: "flag", WantSetType: gjson.True, Nilable: true},
	}
	// 三态键为 null：默认态通过（既有语义），有值态红、空值态也通过。
	body := `{"id":1,"flag":null}`
	if err := apiResponseKeysErr(body, "", want); err != nil {
		t.Fatalf("默认态入口应通过，err = %v", err)
	}
	if err := apiResponseKeysErrInState(body, "", want, KeyStateAny); err != nil {
		t.Fatalf("KeyStateAny 应通过，err = %v", err)
	}
	if err := apiResponseKeysErrInState(body, "", want, KeyStateUnset); err != nil {
		t.Fatalf("KeyStateUnset 应通过，err = %v", err)
	}
	if err := apiResponseKeysErrInState(body, "", want, KeyStateSet); err == nil {
		t.Fatal("KeyStateSet 应报红")
	}
}

// TestAPIResponseKeysErr_AlwaysNull 锁恒 null 键（gorm 软删位这类 ORM 托管字段）的语义：
// 三种态下恒 null 都通过（它不是三态键，没有「有值态」可言）、一旦有值即红；
// 表里标了 AlwaysNull 却不标 Nilable 属表自相矛盾，须当场报错而不是静默放行 null。
func TestAPIResponseKeysErr_AlwaysNull(t *testing.T) {
	want := []APIResponseKey{
		{Key: "id", WantSetType: gjson.Number},
		{Key: "deleted_at", WantSetType: gjson.String, Nilable: true, AlwaysNull: true},
	}
	nullBody := `{"id":1,"deleted_at":null}`
	valuedBody := `{"id":1,"deleted_at":"2026-10-08T00:00:00+08:00"}`

	for _, state := range []KeyState{KeyStateAny, KeyStateSet, KeyStateUnset} {
		if err := apiResponseKeysErrInState(nullBody, "", want, state); err != nil {
			t.Fatalf("state=%v 下恒 null 键应通过，err = %v", state, err)
		}
		// 恒 null 键有值 = ORM 托管形态变了：任何态下都该红
		// （尤其 KeyStateUnset——它不是被漏掉的空值态三态键，豁免不能把它变成哑巴）。
		if err := apiResponseKeysErrInState(valuedBody, "", want, state); err == nil {
			t.Fatalf("state=%v 下恒 null 键有值应报红", state)
		}
	}

	bad := []APIResponseKey{{Key: "deleted_at", WantSetType: gjson.String, AlwaysNull: true}}
	err := apiResponseKeysErrInState(nullBody, "", bad, KeyStateAny)
	if err == nil || !strings.Contains(err.Error(), "自相矛盾") {
		t.Fatalf("err = %v, want 含「自相矛盾」", err)
	}
}
