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
// 判什么：非测试代码里**拿上游响应的状态码跟 2xx 成功码直接比较**——即自行决定
// 「这次算不算通过」——一律红；唯一入口是 common.IsPassingStatus。
//
// 为什么要有这个门：这条判定曾在全项目被写成 7 份，其中 5 份只认正好 200，与另外
// 2 份的 2xx 口径分叉；后果不是诊断文案不一致，而是回 201 的兼容网关被健康检查判死、
// 连带自动停用模型-供应商关联。上一轮（2026-09-12）刚把三份重复的探针编排合并掉，
// 漂移立刻在下一个位置复发——说明「写在文档里靠自觉」挡不住，故编进 `go test ./...`。
//
// 判据边界（刻意不收的）：按**具体失败码**分类的写法合法（401/403 归鉴权、429 归限流、
// 5xx 归供应商），那是「失败之后怎么处置」的另一个决定，不在「算不算通过」这条线上；
// 管理端自己对外回的状态码（httpresp / `c.JSON(http.StatusOK,…)`）不是上游响应，不命中。
//
// 与 gate_500_test.go 同为粗筛网：换个变量名接住状态码再比较（`code := res.StatusCode`）
// 仍能命中 `statusCode` 这一常见命名，但不保证穷尽——评审兜剩下的。
// 判据提为包级变量：既供本门扫描，也供下面的判据样例测试直接复用
// （判据写错 = 门形同虚设，必须有测试钉住它本身）。
var (
	// 成功侧比较基准：2xx 语义常量（含区间上界 300——写 `>= http.StatusMultipleChoices`
	// 同样是自行定义通过区间）与三位数字面量。
	upstreamPassCodes = `http\.Status(?:OK|Created|Accepted|NonAuthoritativeInfo|NoContent|ResetContent|PartialContent|MultipleChoices)|\b2\d{2}\b`
	// 被比较对象：上游响应状态码字段，以及常见的局部变量名 statusCode。
	upstreamPassTarget = `(?:\w*\.StatusCode|\bstatusCode\b)`
	reUpstreamTarget   = regexp.MustCompile(upstreamPassTarget + `\s*(?:==|!=|<=|>=|<|>)\s*(?:` + upstreamPassCodes + `)`)
	reUpstreamCodeHead = regexp.MustCompile(`(?:` + upstreamPassCodes + `)\s*(?:==|!=|<=|>=|<|>)\s*` + upstreamPassTarget)
)

func TestGateUpstreamPassingStatusSingleSource(t *testing.T) {
	// 豁免表：文件（ToSlash 相对路径）→ 理由。列了却不再命中的条目同样判失败，
	// 免得清单悄悄过期成「谁都能往里塞一行」的橡皮图章。
	exemptions := map[string]string{
		"common/upstream_status.go": "通过口径的定义本体，它自己就是那段区间比较",
	}

	// 遍历 + 空转哨兵全门共用（见 gate_sources_test.go）：门扫不到文件就等于没执法，
	// 这类缺陷只在共用实现里修一次守得住。
	files := gateSourceFiles(t, 100)

	var violations []string
	used := make(map[string]bool, len(exemptions))
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s: %v", f, err)
		}
		key := f // gateSourceFiles 已统一为 ToSlash 相对路径
		for i, line := range strings.Split(string(src), "\n") {
			if !reUpstreamTarget.MatchString(line) && !reUpstreamCodeHead.MatchString(line) {
				continue
			}
			if _, ok := exemptions[key]; ok {
				used[key] = true
				continue
			}
			violations = append(violations, fmt.Sprintf("%s:%d 自行判定上游是否成功（应走 common.IsPassingStatus）: %s",
				key, i+1, strings.TrimSpace(line)))
		}
	}
	for path, reason := range exemptions {
		if !used[path] {
			t.Errorf("豁免条目已失效，请从 gate_upstream_status_test.go 的豁免表删掉：%s（登记理由：%s）", path, reason)
		}
	}
	if len(violations) > 0 {
		t.Errorf("上游状态码通过口径未收口到 common.IsPassingStatus（%d 处）:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// TestGateUpstreamPassingStatusJudgment 钉判据本身：门漏判（该红不红）与误判
// （不该红却红）都是这台执法装置的缺陷，光跑真源码只能证明「现在恰好没红」。
// 已知且刻意接受的漏判：把状态码换个名字接住再比较（`code := res.StatusCode` +
// `code == 200`）——RE2 无类型信息，收口这一类的代价是把大量合法写法一起卷进来。
func TestGateUpstreamPassingStatusJudgment(t *testing.T) {
	mustCatch := []string{
		"	if res.StatusCode != http.StatusOK {",
		"	if resp.StatusCode == 200 {",
		"	if r.StatusCode < http.StatusOK || r.StatusCode >= http.StatusMultipleChoices {",
		"	return statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices",
		"	if http.StatusOK != res.StatusCode {",
		"	if res.StatusCode != 201 {",
	}
	mustPass := []string{
		"	if res.StatusCode == http.StatusTooManyRequests {",
		"	if res.StatusCode == http.StatusUnauthorized || res.StatusCode == http.StatusForbidden {",
		"	case statusCode >= 400 && statusCode < 500:",
		"	c.JSON(http.StatusOK, Response{",
		"		return nil, fmt.Errorf(\"status code: %d, response: %s\", res.StatusCode, string(bodyBytes))",
	}

	for _, line := range mustCatch {
		if !reUpstreamTarget.MatchString(line) && !reUpstreamCodeHead.MatchString(line) {
			t.Errorf("判据漏判（应红不红）: %s", strings.TrimSpace(line))
		}
	}
	for _, line := range mustPass {
		if reUpstreamTarget.MatchString(line) || reUpstreamCodeHead.MatchString(line) {
			t.Errorf("判据误判（合法写法被卷进）: %s", strings.TrimSpace(line))
		}
	}
}
