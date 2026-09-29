package settings

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/qkf688/llmux/models"
)

// Reader 是热路径只读配置接口，语义与 models.GetSetting* 完全对齐：
//   - key 不存在 / 解析失败 → 返回调用方传入的 defaultValue
//   - Int 的 minValue=0 表示不强制最小值约束
//
// 业务包（如 chat）通过此接口获取配置，不再直接依赖 Setting 表与 models.GetSetting* 全局函数。
//
// BoolE / IntE 是同取值语义的「读失败可辨」变体（Stage 33）：key 不存在 → (default, nil)
// （未设置是有效状态）；底层查询失败或存值非法 → (default, err)。**「读失败会引发写动作」
// 的决策点必须用 E 变体**并按 err 显式降级（跳过该写动作 + 留痕），禁止拿 default 冒充
// 真配置去执行写——结算序列中途预算失效退默认值曾会误停用可用关联。
type Reader interface {
	Bool(ctx context.Context, key string, defaultValue bool) bool
	Int(ctx context.Context, key string, defaultValue, minValue int) int
	String(ctx context.Context, key string, defaultValue string) string
	BoolE(ctx context.Context, key string, defaultValue bool) (bool, error)
	IntE(ctx context.Context, key string, defaultValue, minValue int) (int, error)
}

// modelsReader 委托 models.GetSetting* 实现 Reader。
// 保留与旧热路径完全一致的默认值 / minValue 策略，零行为漂移。
type modelsReader struct{}

// ModelsReader 是基于 models.GetSetting* 的默认单例 Reader。
//   - models.DB == nil 时 models.GetSetting* 会返回 default，无需额外处理
//   - 测试如需 mock，通过业务包的 SetSettingsReader 注入自定义实现
var ModelsReader Reader = modelsReader{}

func (modelsReader) Bool(ctx context.Context, key string, defaultValue bool) bool {
	return models.GetSettingBool(ctx, key, defaultValue)
}

func (modelsReader) Int(ctx context.Context, key string, defaultValue, minValue int) int {
	return models.GetSettingInt(ctx, key, defaultValue, minValue)
}

func (modelsReader) String(ctx context.Context, key string, defaultValue string) string {
	return models.GetSettingString(ctx, key, defaultValue)
}

func (modelsReader) BoolE(ctx context.Context, key string, defaultValue bool) (bool, error) {
	return models.GetSettingBoolE(ctx, key, defaultValue)
}

func (modelsReader) IntE(ctx context.Context, key string, defaultValue, minValue int) (int, error) {
	return models.GetSettingIntE(ctx, key, defaultValue, minValue)
}

// LogReadSkip 记录「设置读取失败 → 依赖该配置的写动作被跳过」（Stage 33 显式降级口径）。
// 跳过与写库失败都要留痕：否则事后无法回答「漏了多少次账、漏的哪一步」。
// 消息形状单源在此，各结算域（adjustment / chat 凭据结算）共用，禁散落自造文案。
//
// 留痕密度按设置项限频（Stage 36）：调用点遍布结算与健康检测，一次请求会走过多个
// 「读配置 → 写库」步骤，逐步各打一条会让日志量 ≈ 请求量 × 步数——恰在数据库不稳、
// 最需要日志的时刻把有用信息淹没（与本项目「整行限长防刷屏」同取向）。
// 窗口内被压掉的条数在下一条留痕里带出（`suppressed`）：故障持续期总次数可累加还原；
// 但**该窗口内没有下一条**（故障恰好在此窗口结束、或进程重启）时，这段计数就没有下文，
// 属已告知的口径（要精确总数须另设计数通道，见 settings 模块文档）。
func LogReadSkip(step, key string, err error) {
	suppressed, ok := skipLogLimiter.allow(key)
	if !ok {
		return
	}

	slog.Warn("settings read failed: dependent write step skipped",
		"step", step, "key", key, "suppressed", suppressed, "error", err)
}

// skipLogWindow 是同一设置项两条留痕之间的最小间隔。
const skipLogWindow = 60 * time.Second

// readSkipLimiter 是「按设置项限频 + 累计被压条数」的进程内状态。
// 进程重启后计数归零，属已告知的代价（重启前被压掉的条数不再有下文）。
type readSkipLimiter struct {
	mu   sync.Mutex
	last map[string]time.Time
	// dropped 记录窗口内被压掉的条数，下一条留痕带出后归零。
	dropped map[string]int
}

// now 可注入，供测试推快时间（禁在用例里真等一个窗口）。
var skipLogNow = time.Now

var skipLogLimiter = &readSkipLimiter{
	last:    map[string]time.Time{},
	dropped: map[string]int{},
}

// allow 判断该设置项此刻能否出留痕；不能则累加被压条数。
// 返回的 int 是本次留痕要带出的「被压掉条数」（首次/超窗为 0）。
func (l *readSkipLimiter) allow(key string) (int, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := skipLogNow()
	if last, ok := l.last[key]; ok && now.Sub(last) < skipLogWindow {
		l.dropped[key]++
		return 0, false
	}

	dropped := l.dropped[key]
	l.dropped[key] = 0
	l.last[key] = now
	return dropped, true
}

// resetReadSkipLimiter 清空限频状态。
func resetReadSkipLimiter() {
	skipLogLimiter.mu.Lock()
	defer skipLogLimiter.mu.Unlock()

	skipLogLimiter.last = map[string]time.Time{}
	skipLogLimiter.dropped = map[string]int{}
}

// ResetReadSkipLimiterForTest 清空留痕限频状态，仅供测试。
//
// 为什么必须导出：限频状态是进程级的，跨用例（以及 -count=N 的重复轮次）会互相污染
// ——上一轮的窗口计数会把下一轮的留痕压掉，断言留痕的用例就会假红/假绿
// （与 Stage 41 Assemble 跨 count 累积假红同型）。
func ResetReadSkipLimiterForTest() {
	resetReadSkipLimiter()
}
