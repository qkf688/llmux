package balancer

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// AffinityKey 计算「对话身份」键：model + system 段 + tools + 首条消息的语义值摘要。
//
// 为什么是这四个值：供应商提示词缓存按 token 前缀分槽，多轮对话逐轮只在消息数组
// 尾部追加，头部（model / system 段 / tools / 首个用户回合）不变——它们就是缓存前缀的
// 稳定部分，也是区分两个对话的最小身份。调用方（service/chat 的 Beforer）传入规范化
// JSON 语义值而非原始字节：客户端字段顺序、注入字段（stream_options）不改变语义值，
// 键不随序列化形态漂移；语义真变了（换 system/tools/首条消息）即视为新对话，无害退化
// 为不粘（回到现状的随机/轮询选择），不会钉错目标。
//
// 各段长度前缀拼接后再哈希：防相邻两段边界撞车（"ab"+"c" 与 "a"+"bc" 拼接同形）。
func AffinityKey(model string, system, tools, firstMessage []byte) string {
	h := sha256.New()
	writePart := func(b []byte) {
		h.Write(strconv.AppendInt(nil, int64(len(b)), 10))
		h.Write([]byte{':'})
		h.Write(b)
		h.Write([]byte{';'})
	}
	writePart([]byte(model))
	writePart(system)
	writePart(tools)
	writePart(firstMessage)
	return hex.EncodeToString(h.Sum(nil))
}
