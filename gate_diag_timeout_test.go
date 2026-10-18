package main

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 机械门（Stage 19）：诊断族禁「裸写时间字面量」。
//
// 动机（find-issues 2026-09-17-检测类各自写超时与上限）：健康检查与两处 Models() 各写一份
// 30*time.Second，今天恰好一致、明天任何一处单独改值，就会出现「同一条链路探针说超时、
// 健康检查说成功」这类互相矛盾的结论。只靠「新增时记得复用」挡不住——参数必须单源，
// 漂移只剩「刻意改常量」这一处。
//
// 判据（文本行级）：service/healthcheck、service/credprobe、providers、handler/testapi、
// handler/pools 的非测试 .go 中，代码部分（`//` 注释剥除后）出现时间单位字面量
// （time.Second/Millisecond/Microsecond/Nanosecond/Minute/Hour）或 time.Duration( 转换
// 即违规——**含 const/var 声明行**：诊断族的时间预算一律集中到 common/upstream_timeout.go
// 单源，例外必须显式。唯一放行 = 行级标记 `//gate:diag-timeout-allow <非空理由>`：
// 标记了却不再违规同样判红（防注释沉积）；不做文件级豁免（同 gate_upstream_status 口径）。
//
// 已知且刻意接受的漏判：① 无单位裸数字（`Timeout: 30`）与字符串参数的 ParseDuration 变量
// 传值不命中（判不出语义）；② 跨行表达式不命中（本门按行判，gate_upstream_status 按折叠
// 源码判跨行——两门口径不同，此处刻意求简）；③ 字符串字面量里藏时间表达式会被误判违规、
// `//` 出现在字符串中间时注释剥除会截早漏判；④ `/* */` 块注释不剥，块注释里的示例会误判。
// 以上都可用行级标记收口——机械门是快检不是证明。

// diagTimeoutMarkerBase 是行级放行标记前缀，理由跟在后面；允许裸标记被识别为 reasonless 判红。
const diagTimeoutMarkerBase = "//gate:diag-timeout-allow"

// diagTimeoutHome 是诊断类时间预算的**唯一**合法住所；diagTimeoutDirs 同时覆盖诊断请求
// 五族（健康检查 / 凭据探测 / providers 拉取 / 连通测试 / pools 按凭据测试）与 common 整域
// ——后者是防止「预算值写到 upstream_timeout.go 之外」的另一半守望。
const diagTimeoutHome = "common/upstream_timeout.go"

var diagTimeoutDirs = []string{
	"common",
	"service/healthcheck",
	"service/credprobe",
	"service/protocolprobe",
	"providers",
	"handler/testapi",
	"handler/pools",
}

var (
	reTimeUnit       = regexp.MustCompile(`\btime\.(Second|Millisecond|Microsecond|Nanosecond|Minute|Hour)\b`)
	reTimeDurationFn = regexp.MustCompile(`\btime\.(Duration|ParseDuration)\(`)
)

// diagTimeoutConstants 是诊断类时间预算的单源常量（common/upstream_timeout.go）。
// 正向哨兵：常量被删/被改名会让调用点回归裸写字面量，门必须同时锁「单源文件在、名字在」。
var diagTimeoutConstants = []string{
	"DiagnosticResponseHeaderTimeout",
	"DiagnosticModelsProbeTimeout",
	"DiagnosticCompletionTimeout",
	"DiagnosticTransportBuffer",
	"DiagnosticCredentialProbeTimeout",
	"DiagnosticConnectivityTestTimeout",
	"DiagnosticReactHeaderTimeout",
	"DiagnosticProtocolProbeTimeout",
}

// diagTimeoutLineStatus 判单行。返回：ok / violation（裸写）/ reasonless（标记缺理由）/
// stale（标记失效：带标记却没有违规可放行）。
//
// 标记的两个边界（Stage 19 审查回批 Minor 7）：① 标记前缀必须以行尾或空白收尾——
// `allowX 任意文字` 这类伪标记不算标记，不得放行该行违规；② 标记只认**代码行的尾注释**——
// 纯注释行里的标记文本是教用法，不构成标记，也不该被判 stale。
func diagTimeoutLineStatus(raw string) string {
	code := raw
	if i := strings.Index(code, "//"); i >= 0 {
		code = code[:i]
	}
	marked := false
	reason := ""
	if idx := strings.Index(raw, diagTimeoutMarkerBase); idx >= 0 {
		rest := raw[idx+len(diagTimeoutMarkerBase):]
		if (rest == "" || rest[0] == ' ' || rest[0] == '\t') && strings.TrimSpace(code) != "" {
			marked = true
			reason = strings.TrimSpace(rest)
		}
	}
	violated := reTimeUnit.MatchString(code) || reTimeDurationFn.MatchString(code)
	switch {
	case violated && !marked:
		return "violation"
	case violated && reason == "":
		return "reasonless"
	case !violated && marked:
		return "stale"
	default:
		return "ok"
	}
}

// TestGateDiagTimeoutLiterals 扫五族源码判「裸写时间字面量」。
func TestGateDiagTimeoutLiterals(t *testing.T) {
	all := gateSourceFiles(t, 250)
	inScope := make([]string, 0, 64)
	for _, f := range all {
		// 单源家本身免扫：诊断时间预算只允许活在 common/upstream_timeout.go——这不是
		// 「文件级豁免」，是「值必须住在哪」的另一半判据（审查回批 Minor 6）。
		if f == diagTimeoutHome {
			continue
		}
		for _, dir := range diagTimeoutDirs {
			// dir+".go" 覆盖与目录同名的门面文件（如 service/healthcheck.go），否则漏扫一族。
			if f == dir+".go" || strings.HasPrefix(f, dir+"/") {
				inScope = append(inScope, f)
				break
			}
		}
	}
	if len(inScope) < 40 {
		t.Fatalf("五族目录只扫到 %d 个源文件，低于下限 40——过滤条件写错，本门正在空转", len(inScope))
	}
	for _, anchor := range []string{
		"service/healthcheck/check_single.go",
		"service/credprobe/completion_probe.go",
		"providers/cache.go",
		"handler/testapi/react_handler.go",
		"handler/pools/credential_probe.go",
	} {
		found := false
		for _, f := range inScope {
			if f == anchor {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("扫描范围未覆盖锚点文件 %s——目录过滤条件写错，本门扫不到它背后的整棵子树", anchor)
		}
	}

	type bad struct{ file, line, status, text string }
	var violations []bad
	for _, f := range inScope {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", f, err)
		}
		for i, line := range strings.Split(string(data), "\n") {
			status := diagTimeoutLineStatus(line)
			if status != "ok" {
				violations = append(violations, bad{f, strconv.Itoa(i + 1), status, strings.TrimSpace(line)})
			}
		}
	}
	if len(violations) > 0 {
		var sb strings.Builder
		sb.WriteString("诊断族出现裸写时间字面量（或标记异常），时间预算必须集中到 common/upstream_timeout.go；\n")
		sb.WriteString("确属例外（非诊断超时语义）才允许行级标记 //gate:diag-timeout-allow <理由>：\n")
		for _, v := range violations {
			sb.WriteString("  " + v.file + ":" + v.line + " [" + v.status + "] " + v.text + "\n")
		}
		t.Fatal(sb.String())
	}
}

// TestGateDiagTimeoutConstantsSingleSourced 正向哨兵：单源常量文件与其锚点常量必须在。
func TestGateDiagTimeoutConstantsSingleSourced(t *testing.T) {
	data, err := os.ReadFile("common/upstream_timeout.go")
	if err != nil {
		t.Fatalf("读取 common/upstream_timeout.go 失败（诊断时间预算单源文件缺失）: %v", err)
	}
	src := string(data)
	for _, name := range diagTimeoutConstants {
		// 判「声明」而非「提及」：常量 doc 注释里必然写着自己的名字，Contains 会被
		// 「删声明留注释」骗过（Stage 19 审查回批 Minor 6）。
		if !regexp.MustCompile(`(?m)^\s*` + name + `\s*=`).MatchString(src) {
			t.Fatalf("common/upstream_timeout.go 未声明 %s——单源常量被删/改名会让调用点回归裸写字面量", name)
		}
	}
}

// TestGateDiagTimeoutJudgment 判据自测：钉住不漏判、不误判（同 gate_upstream_status 口径）。
func TestGateDiagTimeoutJudgment(t *testing.T) {
	mustCatch := []string{
		`	httpClient := providers.GetClientWithProxy(30*time.Second, client.GetProxy())`,
		`	httpClient := providers.GetClientWithProxy(time.Second*60, proxyURL)`,
		`		Timeout: 15 * time.Second,`,
		`const credProbeTimeout = 15 * time.Second`,
		`	return time.Duration(minutes) * time.Minute`,
		`		Timeout:   30 * time.Second,`,
		`	s := time.ParseDuration("30s")`,
		`x := 30 * time.Second //gate:diag-timeout-allowX 任意文字`,
	}
	for _, line := range mustCatch {
		if got := diagTimeoutLineStatus(line); got != "violation" {
			t.Fatalf("漏判：diagTimeoutLineStatus(%q) = %s, want violation", line, got)
		}
	}
	mustPass := []string{
		`// 兜底 DefaultCompletionTimeout 口径，此处不写字面量（30 * time.Second 见 common）`,
		`	Timeout time.Duration // 请求整体超时，由调用方显式传`,
		`	since := time.Since(start)`,
		`	return time.Duration(minutes) * time.Minute //gate:diag-timeout-allow settings 分钟值换算，非诊断超时参数`,
		`// 用法：确属例外时行尾加 //gate:diag-timeout-allow <理由>（纯注释提及标记不算标记失效）`,
		`	Timeout: 30, //gate:diag-timeout-allowX 伪标记无分隔符不算标记；本行无违规，判 ok`,
	}
	for _, line := range mustPass {
		if got := diagTimeoutLineStatus(line); got != "ok" {
			t.Fatalf("误判：diagTimeoutLineStatus(%q) = %s, want ok", line, got)
		}
	}
}

// TestGateDiagTimeoutMarker 标记机制三态：缺理由判红、失效判红（防注释沉积）。
func TestGateDiagTimeoutMarker(t *testing.T) {
	if got := diagTimeoutLineStatus(`x := 30 * time.Second //gate:diag-timeout-allow`); got != "reasonless" {
		t.Fatalf("标记缺理由 = %s, want reasonless", got)
	}
	if got := diagTimeoutLineStatus(`x := 1 //gate:diag-timeout-allow 不再违规的旧标记`); got != "stale" {
		t.Fatalf("标记失效 = %s, want stale", got)
	}
	if got := diagTimeoutLineStatus(`	Timeout: 1 * time.Second, //gate:diag-timeout-allow Transport 连接池调优，生产/诊断共用`); got != "ok" {
		t.Fatalf("合法标记放行 = %s, want ok", got)
	}
}
