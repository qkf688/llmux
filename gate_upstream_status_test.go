package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// Stage 5 机械门（约定见 docs/guides/framework-conventions.md §6）。
//
// 判什么：非测试代码里**自行判定上游响应算不算成功**——拿状态码跟成功侧状态码比较、
// 做算术、或按 `Status` 字符串前缀归类——一律红；唯一入口是 common.IsPassingStatus。
//
// 为什么要有这个门：这条判定曾在全项目被写成 7 份，5 份只认正好 200、2 份认 2xx，分叉
// 的后果不是文案不一致，而是回 201 的兼容网关被健康检查判死并**自动停用关联**。上一轮
// （2026-09-12）刚合并完三份探针编排，漂移立刻在新位置复发——「写在文档里靠自觉」挡不住。
//
// 判据边界（刻意不收的）：按**具体失败码**分类合法（401/403 归鉴权、429 归限流、5xx 归
// 供应商，如 `chat_attempt_error.go` 的 429 降权、`common.ErrorTypeFromStatus`），那是
// 「失败之后怎么处置」的另一个决定，不在「算不算通过」这条线上。
//
// 放行方式只有行级标记：违规行行尾写 `//gate:upstream-status-allow <非空理由>`。
// 刻意不做成文件级豁免——整文件免检等于给一处合法例外买下一整座文件的后门。
// 标记了却不再违规同样判红，防止注释沉积。

const (
	// gateUpstreamAllowMarker 行级放行标记（理由必须非空，见 scanUpstreamStatusSource）。
	gateUpstreamAllowMarker = "//gate:upstream-status-allow"

	// 成功侧状态码：2xx 语义常量（含 204/206/区间上下界 300 等）与三位数字面量。
	// 刻意比 IsPassingStatus 的实际通过集（200/201）更宽——本门要禁的是「自行判定」这件事，
	// 不是「判成另一个集合」；写 `== 204`、`>= 200 && < 300` 同样应该被拦下来讨论。
	upstreamPassCodes = `http\.Status(?:OK|Created|Accepted|NonAuthoritativeInfo|NoContent|ResetContent|PartialContent|MultipleChoices)|\b2\d{2}\b`
	upstreamPassCmp   = `(?:==|!=|<=|>=|<|>)`
	// 被比较对象：上游响应状态码字段，以及常见的局部变量名/参数名 statusCode。
	upstreamPassTarget = `(?:\w*\.StatusCode|\bstatusCode\b)`
)

var (
	reUpstreamTargetFirst = regexp.MustCompile(upstreamPassTarget + `\s*` + upstreamPassCmp + `\s*(?:` + upstreamPassCodes + `)`)
	reUpstreamCodeHead    = regexp.MustCompile(`(?:` + upstreamPassCodes + `)\s*` + upstreamPassCmp + `\s*` + upstreamPassTarget)
	// 换写法绕不过去：switch 的 case 直接列成功码、除 100 取百位、按 Status 字符串前缀归类。
	reUpstreamCaseCode  = regexp.MustCompile(`\bcase\s+(?:` + upstreamPassCodes + `)\b`)
	reUpstreamDivided   = regexp.MustCompile(upstreamPassTarget + `\s*/\s*100`)
	reUpstreamStatusPfx = regexp.MustCompile(`HasPrefix\(\s*(?:\w*\.Status|\bstatus)\s*,\s*"2`)
)

// upstreamHit 一处判据命中（含所在行、是否被行级标记放行、标记是否已失效）。
type upstreamHit struct {
	line       int // 1 起
	text       string
	allowed    bool // 违规行带合法标记
	violating  bool // 该行确实命中判据
	reasonless bool // 有标记但没写理由
}

// stale 标记已失效（行上没有违规了，注释还留着）——上层据此要求删注释。
func (h upstreamHit) stale() bool { return h.allowed && !h.violating }

// scanUpstreamStatusSource 对单份源文件跑全部判据。
//
// 跨行写法（`res.StatusCode !=\n http.StatusOK`）怎么抓：把所有行用一个空格拼成一条再匹配，
// 命中位置按累计长度折回行号。逐行喂会把这种写法静默放过——`\s*` 本来就允许换行，
// 不拼起来等于自废这条能力。
func scanUpstreamStatusSource(src string) []upstreamHit {
	lines := strings.Split(src, "\n")
	folded := strings.Join(lines, " ")

	// 折叠串中的偏移 → 原行号：第 i 行在折叠串中占 [off_i, off_i+len(line_i)+1)。
	offsets := make([]int, len(lines))
	acc := 0
	for i, l := range lines {
		offsets[i] = acc
		acc += len(l) + 1
	}
	lineOf := func(pos int) int {
		lo, hi := 0, len(offsets)-1
		for lo < hi {
			mid := (lo + hi + 1) / 2
			if offsets[mid] <= pos {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		return lo + 1
	}

	byLine := make(map[int]bool)
	for _, re := range []*regexp.Regexp{reUpstreamTargetFirst, reUpstreamCodeHead, reUpstreamCaseCode, reUpstreamDivided, reUpstreamStatusPfx} {
		for _, m := range re.FindAllStringIndex(folded, -1) {
			byLine[lineOf(m[0])] = true
		}
	}
	if len(byLine) == 0 && !strings.Contains(src, gateUpstreamAllowMarker) {
		return nil
	}

	hits := make([]upstreamHit, 0, len(byLine))
	for i, raw := range lines {
		line := i + 1
		marked := strings.Contains(raw, gateUpstreamAllowMarker)
		violating := byLine[line]
		if !violating && !marked {
			continue
		}
		hits = append(hits, upstreamHit{
			line:      line,
			text:      strings.TrimSpace(raw),
			violating: violating,
			allowed:   marked,
			// 标记了但没写理由（标记之后只剩空白）= 悄悄开后门。
			reasonless: marked && strings.TrimSpace(strings.SplitN(raw, gateUpstreamAllowMarker, 2)[1]) == "",
		})
	}
	return hits
}

func TestGateUpstreamPassingStatusSingleSource(t *testing.T) {
	// 下限按当前源码规模留了余量；锚点文件断言在 gate_sources_test.go。
	files := gateSourceFiles(t, 250)

	var violations []string
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s: %v", f, err)
		}
		for _, hit := range scanUpstreamStatusSource(string(src)) {
			switch {
			case hit.reasonless:
				violations = append(violations, fmt.Sprintf("%s:%d 放行标记缺理由（必须写清为什么合法）: %s", f, hit.line, hit.text))
			case hit.stale():
				violations = append(violations, fmt.Sprintf("%s:%d 放行标记已失效（该行已无违规），请删掉标记: %s", f, hit.line, hit.text))
			case !hit.allowed:
				violations = append(violations, fmt.Sprintf("%s:%d 自行判定上游是否成功（应走 common.IsPassingStatus）: %s", f, hit.line, hit.text))
			}
		}
	}
	if len(violations) > 0 {
		t.Errorf("上游状态码通过口径未收口到 common.IsPassingStatus（%d 处）:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// TestGateUpstreamPassingStatusJudgment 钉判据本身：门漏判（该红不红）与误判（不该红却红）
// 都是这台执法装置的缺陷，光跑真源码只能证明「现在恰好没红」。
// 已知且刻意接受的漏判：把状态码改名后再比较（`code := res.StatusCode; code == 200`）——
// RE2 没有类型信息，收尽这一类的代价是把大量合法写法卷进来，留给评审兜。
func TestGateUpstreamPassingStatusJudgment(t *testing.T) {
	mustCatch := []struct {
		name string
		src  string
	}{
		{"直接比较", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode != http.StatusOK\n}\n"},
		{"字面量 200", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode == 200\n}\n"},
		{"反向写法", "package x\n\nfunc f(res *http.Response) bool {\n\treturn http.StatusCreated != res.StatusCode\n}\n"},
		{"区间自定", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode >= http.StatusOK && res.StatusCode < http.StatusMultipleChoices\n}\n"},
		{"跨行比较", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode !=\n\t\thttp.StatusOK\n}\n"},
		{"switch case 列成功码", "package x\n\nfunc f(res *http.Response) bool {\n\tswitch res.StatusCode {\n\tcase http.StatusOK, http.StatusCreated:\n\t\treturn true\n\t}\n\treturn false\n}\n"},
		{"除 100 取百位", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode/100 == 2\n}\n"},
		{"按 Status 字符串前缀", "package x\n\nfunc f(res *http.Response) bool {\n\treturn strings.HasPrefix(res.Status, \"2\")\n}\n"},
		{"局部变量名 statusCode", "package x\n\nfunc f(statusCode int) bool {\n\treturn statusCode == 201\n}\n"},
	}
	for _, tc := range mustCatch {
		hits := scanUpstreamStatusSource(tc.src)
		if len(hits) == 0 {
			t.Errorf("判据漏判（应红不红）：%s", tc.name)
			continue
		}
		for _, h := range hits {
			if h.allowed {
				t.Errorf("判据自造豁免：%s 第 %d 行未标标记却判放行", tc.name, h.line)
			}
		}
	}

	mustPass := []struct {
		name string
		src  string
	}{
		{"失败码分类 429", "package x\n\nfunc f(res *http.Response) string {\n\tswitch res.StatusCode {\n\tcase http.StatusTooManyRequests:\n\t\treturn \"rate\"\n\t}\n\treturn \"\"\n}\n"},
		{"失败码比较 401/403", "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden\n}\n"},
		{"4xx 区间归类", "package x\n\nfunc f(statusCode int) string {\n\tif statusCode >= 400 && statusCode < 500 {\n\t\treturn \"validation\"\n\t}\n\treturn \"\"\n}\n"},
		{"管理端自己回 200", "package x\n\nfunc f(c *gin.Context) {\n\tc.JSON(http.StatusOK, Response{Code: http.StatusOK})\n}\n"},
		{"错误文本带状态码", "package x\n\nfunc f(res *http.Response) error {\n\treturn fmt.Errorf(\"status code: %d, response: %s\", res.StatusCode, body)\n}\n"},
	}
	for _, tc := range mustPass {
		if hits := scanUpstreamStatusSource(tc.src); len(hits) > 0 {
			t.Errorf("判据误判（合法写法被卷进）：%s → %+v", tc.name, hits)
		}
	}
}

// TestGateUpstreamPassingStatusMarker 钉放行机制：标记必须真的写对理由，且不允许沉积。
func TestGateUpstreamPassingStatusMarker(t *testing.T) {
	violating := "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode == http.StatusOK //gate:upstream-status-allow 本端点协议规定非 200 即错\n}\n"
	hits := scanUpstreamStatusSource(violating)
	if len(hits) != 1 || !hits[0].allowed || hits[0].reasonless {
		t.Fatalf("带理由的标记行应被判「已放行」，got %+v", hits)
	}

	reasonless := "package x\n\nfunc f(res *http.Response) bool {\n\treturn res.StatusCode == http.StatusOK //gate:upstream-status-allow\n}\n"
	hits = scanUpstreamStatusSource(reasonless)
	if len(hits) != 1 || !hits[0].reasonless {
		t.Fatalf("标记无应判「缺理由」，got %+v", hits)
	}

	stale := "package x\n\nfunc f(res *http.Response) bool {\n\treturn !common.IsPassingStatus(res.StatusCode) //gate:upstream-status-allow 历史遗留\n}\n"
	hits = scanUpstreamStatusSource(stale)
	if len(hits) != 1 || !hits[0].stale() {
		t.Fatalf("标记行（该行已无违规）必须被报出来供上层判失效，got %+v", hits)
	}
}
