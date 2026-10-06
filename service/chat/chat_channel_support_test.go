package chat

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/qkf688/llmux/common/credentialcrypto"
	"github.com/qkf688/llmux/consts"
	"github.com/qkf688/llmux/models"
	"github.com/qkf688/llmux/providers"
)

// setupChatChannelCipher 注入凭据加密 cipher（选路需要解密明文 key）。
// 与 service/channel 测试同姿势：SetDefault + t.Cleanup 复位，测试间不残留。
// ⚠️ 本包测试基于 credentialcrypto 包级全局注入，**禁止为 chat 包测试加 t.Parallel()**
// （多个并行用例会交叉污染同一全局 cipher/repository 默认实例）。
func setupChatChannelCipher(t *testing.T) *credentialcrypto.Cipher {
	t.Helper()
	cipher, err := credentialcrypto.New(testChatCredHexKey)
	if err != nil {
		t.Fatalf("new cipher: %v", err)
	}
	credentialcrypto.SetDefault(cipher)
	t.Cleanup(func() { credentialcrypto.SetDefault(nil) })
	return cipher
}

// mustEncrypt 加密测试凭据 key（helper，避免每个用例重复 error 样板）。
// 接口参数保持最小面：只需 Encrypt，解密/哈希由调用方直接用 *credentialcrypto.Cipher。
func mustEncrypt(t *testing.T, cipher interface {
	Encrypt(plain string) (string, error)
}, plain string) string {
	t.Helper()
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt %q: %v", plain, err)
	}
	return enc
}

// chatTestPoolSeq 号池名序号：Pool.Name 是唯一索引，同一用例内多次建池（如表驱动子用例
// 各自建组）必须靠序号区分，否则第二条插入撞唯一约束。
var chatTestPoolSeq atomic.Int64

// seedPooledGroup 建「号池 + 引用该池的分组」，返回分组与其池 ID。
// 内联凭据概念已删除：分组只能经 KeyGroup.PoolID 引用号池取凭据
// （见 service/channel/assemble.go 的 groupCredentials），凭据一律写 PoolID。
func seedPooledGroup(t *testing.T, providerID uint, name string, weight int, whitelist string) (models.KeyGroup, uint) {
	t.Helper()
	pool := models.Pool{Name: fmt.Sprintf("pool-%s-%d", name, chatTestPoolSeq.Add(1))}
	if err := models.DB.Create(&pool).Error; err != nil {
		t.Fatalf("create pool: %v", err)
	}
	grp := models.KeyGroup{ProviderID: providerID, Name: name, Weight: weight, Models: whitelist, PoolID: &pool.ID}
	if err := models.DB.Create(&grp).Error; err != nil {
		t.Fatalf("create key group %s: %v", name, err)
	}
	return grp, pool.ID
}

// seedPoolCredentials 往 seedPooledGroup 建出的池里落凭据。keys 传**明文**：
// 加密与 KeyHash 都在这里算，调用方只描述状态机字段（Status / LastProbeAt /
// CooldownReason 等逐用例不同，不猜）。凭据一律写 PoolID——内联凭据概念已删除。
func seedPoolCredentials(t *testing.T, cipher *credentialcrypto.Cipher, poolID uint, keys []string, mutate func(*models.Credential)) {
	t.Helper()
	for i, key := range keys {
		cred := models.Credential{
			PoolID:  poolID,
			Key:     mustEncrypt(t, cipher, key),
			KeyHash: cipher.Hash(key),
			Status:  models.CredentialStatusActive,
		}
		if mutate != nil {
			mutate(&cred)
		}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential #%d: %v", i, err)
		}
	}
}

// seedGroupedChannel 收敛「号池 + 引用池的分组 + 池内凭据 + 模型关联」这段夹具样板：
// 内联凭据删除后每个选路用例都要现建池/组/凭据/关联四层，逐个用例手写会在多个测试文件
// 里长出逐字重复。mutate 用于给凭据补状态机字段（nil 表示全 active）。
// 返回 (分组, 池ID, 关联)——关联回填了 ID，调用方构造 CandidatePool 要用。
func seedGroupedChannel(
	t *testing.T,
	cipher *credentialcrypto.Cipher,
	providerID uint,
	groupName string,
	modelID uint,
	weight, priority int,
	keys []string,
	mutate func(*models.Credential),
) (models.KeyGroup, uint, models.ModelWithProvider) {
	t.Helper()
	grp, poolID := seedPooledGroup(t, providerID, groupName, weight, "")
	seedPoolCredentials(t, cipher, poolID, keys, mutate)
	mwp := models.ModelWithProvider{ModelID: modelID, ProviderID: providerID, ProviderModel: "pm1", Weight: weight, Priority: priority}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	return grp, poolID, mwp
}

// seedProbeRecoveryCredentials 建「惰性探活恢复」场景的两条凭据：
// key1 active 且 LastProbeAt 落在 60s 频控窗口内（401 判停后探活跳过它），
// key2 已判停等待探活恢复（探活候选 ID ASC，key1 被跳过后才轮到它）。
// 两个探活用例共用这同一形态，手写会在各自文件里长出逐字重复；
// 返回落库后的两条凭据 ID（用例要 reload 断言判停/恢复后的状态变化）。
func seedProbeRecoveryCredentials(t *testing.T, cipher *credentialcrypto.Cipher, poolID uint) (uint, uint) {
	t.Helper()
	probedRecently := time.Now().Add(-30 * time.Second)
	ids := make([]uint, 0, 2)
	for _, key := range []string{"sk-key1", "sk-key2"} {
		cred := models.Credential{
			PoolID:  poolID,
			Key:     mustEncrypt(t, cipher, key),
			KeyHash: cipher.Hash(key),
			Status:  models.CredentialStatusActive,
		}
		if key == "sk-key1" {
			cred.LastProbeAt = &probedRecently
		} else {
			cred.Status = models.CredentialStatusTempUnsched
			cred.CooldownReason = "auth_fail"
			cred.FailCount = 3
		}
		if err := models.DB.Create(&cred).Error; err != nil {
			t.Fatalf("create credential %s: %v", key, err)
		}
		ids = append(ids, cred.ID)
	}
	return ids[0], ids[1]
}

// seedProbeChannel 收敛「1 provider + 1 端点 + 1 号池分组 + 凭据 + 1 关联 +
// 组装好的 CandidatePool」这段探活用例夹具：探活/重试用例的入参形状逐字相同
// （区别只在 srv 的回包与凭据状态），手写会在本文件里长出逐字重复。
// whitelist 非空时写进分组白名单；weight/priority 直传关联与 CandidatePool 的
// WeightItems/PriorityItems。seedCred 落池内凭据并返回两个凭据 ID（用例要 reload
// 断言判停/恢复后的状态）。
func seedProbeChannel(
	t *testing.T,
	cipher *credentialcrypto.Cipher,
	providerName string,
	baseURL string,
	modelID uint,
	whitelist string,
	weight, priority int,
	seedCred func(t *testing.T, poolID uint) (uint, uint),
) (models.Provider, models.ModelWithProvider, CandidatePool, uint, uint) {
	t.Helper()
	provider := models.Provider{Name: providerName, Type: providers.TypeOpenAI, Config: fmt.Sprintf(`{"base_url":%q}`, baseURL)}
	if err := models.DB.Create(&provider).Error; err != nil {
		t.Fatalf("create provider: %v", err)
	}
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOpenAI), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint: %v", err)
	}
	_, poolID := seedPooledGroup(t, provider.ID, "g1", 1, whitelist)
	id1, id2 := seedCred(t, poolID)
	mwp := models.ModelWithProvider{ModelID: modelID, ProviderID: provider.ID, ProviderModel: "pm1", Weight: weight, Priority: priority}
	if err := models.DB.Create(&mwp).Error; err != nil {
		t.Fatalf("create association: %v", err)
	}
	pool := CandidatePool{
		ProviderMap: map[uint]models.Provider{
			provider.ID: provider,
		},
		ModelWithProviderMap: map[uint]models.ModelWithProvider{
			mwp.ID: mwp,
		},
		WeightItems:   map[uint]int{mwp.ID: weight},
		PriorityItems: map[uint]int{mwp.ID: priority},
	}
	return provider, mwp, pool, id1, id2
}

// runProbeRetryLoop 收敛探活/重试用例逐字相同的 retryLoopInput 构造：三个用例的
// 区别只在 srv 的回包与凭据状态，入参形状完全一致（模型 m、上游窗口 2s、MaxRetry 5）。
func runProbeRetryLoop(pool CandidatePool) retryLoopOutcome {
	return runProviderRetryLoop(retryLoopInput{
		Ctx:            context.Background(),
		Start:          time.Now(),
		Style:          string(consts.StyleOpenAI),
		Before:         Before{Model: "m", Stream: false, raw: []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}]}`)},
		RealModelName:  "m",
		Pool:           pool,
		MaxRetry:       5,
		UpstreamWindow: upstreamWindow{Window: 2 * time.Second},
	})
}

// addLegacyChannelRows 给 provider 补「S1 迁移后老 Provider 等价形态」的选路数据：
// 1 端点（协议=ProtocolOfType(type)、URL 空继承 Provider.Config.base_url）
// + 1 号池 + 1 引用该池的分组 + 1 池内凭据（明文 key 经 cipher 加密落库）。
// S3-2 起凭据只存在加密的 credentials 表，直连 Config.api_key 的旧测试数据必须补行，
// 否则选路在端点/凭据层直接失败。
func addLegacyChannelRows(t *testing.T, cipher *credentialcrypto.Cipher, provider models.Provider) {
	t.Helper()
	ep := models.Endpoint{ProviderID: provider.ID, Protocol: string(consts.ProtocolOfType(provider.Type)), Enabled: true}
	if err := models.DB.Create(&ep).Error; err != nil {
		t.Fatalf("create endpoint for provider %q: %v", provider.Name, err)
	}
	_, poolID := seedPooledGroup(t, provider.ID, "g1", 1, "")
	plain := "test-key" // 任意占位明文：KeyHash 必须与之配对（Hash 加密散发），
	// 导入不同明文时同时改两处，否则选路解密出的 key 与 KeyHash 断言不一致。
	enc, err := cipher.Encrypt(plain)
	if err != nil {
		t.Fatalf("encrypt credential for provider %q: %v", provider.Name, err)
	}
	cred := models.Credential{PoolID: poolID, Key: enc, KeyHash: cipher.Hash(plain)}
	if err := models.DB.Create(&cred).Error; err != nil {
		t.Fatalf("create credential for provider %q: %v", provider.Name, err)
	}
}
