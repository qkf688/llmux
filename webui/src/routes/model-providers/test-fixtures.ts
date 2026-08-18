import type { ModelWithProvider } from "@/lib/api";

/**
 * 关联对象的测试基线，形状必须与后端 `GET /api/model-providers` 的真实响应一致
 * （`models.ModelWithProvider` 被 handler 直返，全字段 PascalCase；三态字段
 * `SupportsThinking` / `ThinkingLevels` 的继承态为 JSON null，键始终存在）。
 *
 * 收口成单一 fixture 的理由：曾出现「前端 mock 按理想形状手写、后端实际响应键名不同」
 * 的契约断层，两侧测试各自通过却在真实链路上失效。契约变更时只改这一处，
 * 所有用例同步暴露。
 */
export function createMockAssociation(
  ID: number,
  ProviderID: number,
  overrides: Partial<ModelWithProvider> = {},
): ModelWithProvider {
  return {
    ID,
    ModelID: 1,
    ProviderModel: "gpt-test",
    ProviderID,
    ToolCall: true,
    StructuredOutput: false,
    Image: false,
    WithHeader: false,
    CustomerHeaders: {},
    Status: true,
    Weight: 1,
    Priority: 0,
    MaxTokens: null,
    SupportsThinking: null,
    ThinkingLevels: null,
    ...overrides,
  };
}
