import { useQuery, useMutation, useQueryClient, type QueryClient } from '@tanstack/react-query';
import { invalidateModelProviders } from '@/hooks/api/use-model-providers';
import {
  getProviders,
  getProviderModelCatalog,
  getProviderTemplates,
  createProvider,
  updateProvider,
  deleteProvider,
  clearProviderAssociations,
  syncAllProviderModels,
  getSettings,
  updateSettings,
  getHealthCheckSettings,
  updateHealthCheckSettings,
  type Provider,
  type ProviderModelCatalog,
  type ProviderTemplate,
  type HealthCheckSettings,
} from '@/lib/api';
import type { Settings } from '@/lib/api';

export const providerKeys = {
  all: ['providers'] as const,
  lists: () => [...providerKeys.all, 'list'] as const,
  list: (filters?: { name?: string; type?: string }) =>
    [...providerKeys.lists(), filters ?? {}] as const,
  templates: () => [...providerKeys.all, 'templates'] as const,
  catalog: () => [...providerKeys.all, 'catalog'] as const,
};

export const settingsKeys = {
  all: ['settings'] as const,
};

export const healthCheckSettingsKeys = {
  all: ['healthCheckSettings'] as const,
};

/** 一次 provider 侧写操作要失效哪些缓存族。缺省 = 最常见的组合。 */
export type ProviderWriteInvalidation = {
  /** 提供商列表。个别入口已用 setQueriesData 本地打补丁，可关掉避免整表重取。 */
  lists?: boolean;
  /** 组织级模型目录（分组白名单并集 ∪ custom）：只有同步 / 改 custom 才需要。 */
  catalog?: boolean;
  /** 模型关联页缓存（modelProviderKeys，另一套 key）。端点确不触发 autoassoc 时才可关。 */
  associations?: boolean;
};

/**
 * 提供商侧写操作后的缓存失效**唯一入口**。
 *
 * 为什么这两个缓存族必须成对：后端对 provider 写路径几乎都会改写 `model_with_providers`
 * （POST/PUT 走 `TriggerAutoAssociate` + `TriggerAutoClean`，DELETE 与「清除关联」直接删行），
 * 而「模型关联」页读的是**独立的** `modelProviderKeys`。全局 `staleTime` 是 60s 且
 * `refetchOnWindowFocus` 已关，漏掉后者的后果是：用户清完关联再进该页仍看见旧行，
 * 以为没生效而重复点击，第二次自然得到 `deleted: 0`（本仓曾如此）。
 *
 * 为什么收成一处而不是各调用点自己写两行：这些入口分散在 6 个文件、各自只看得到自己
 * 那一半缓存；成对写、成对改，漏一处就复现同一类 bug（2026-10 一轮修复连补 7 处即是此因）。
 * 新增 provider 写入口时**一律**走这里，不要自己拼 invalidateQueries。
 *
 * 返回值：所有已完成失效动作的 Promise。需要「refetch 落定后再读缓存」的调用点
 * （如「全部模型」弹窗写完目录立刻回读）必须 await，否则读到的是旧值；
 * 纯事件通知型调用点可以不管（`void` 或不接）。
 */
export function invalidateAfterProviderWrite(
  qc: QueryClient,
  options: ProviderWriteInvalidation = {},
): Promise<unknown> {
  const { lists = true, catalog = false, associations = true } = options;
  const tasks: Promise<unknown>[] = [];

  if (lists) {
    tasks.push(qc.invalidateQueries({ queryKey: providerKeys.lists() }));
  }
  if (catalog) {
    tasks.push(qc.invalidateQueries({ queryKey: providerKeys.catalog() }));
  }
  if (associations) {
    // invalidateModelProviders 本身是同步 fire-and-forget（既有签名），
    // 关联页数据不参与任何「写完立刻回读」的链路，无须纳入 await 集合。
    invalidateModelProviders(qc);
  }

  return Promise.all(tasks);
}

export function useProviders(filters?: { name?: string; type?: string }) {
  return useQuery({
    queryKey: providerKeys.list(filters),
    queryFn: () => getProviders(filters),
  });
}

export function useProviderTemplates() {
  return useQuery({
    queryKey: providerKeys.templates(),
    queryFn: getProviderTemplates,
    staleTime: 5 * 60_000,
  });
}

/** 组织级模型目录（分组白名单并集 + custom）：models / model-providers /
 *  providers 三页的目录数据源。同步或写 custom 后 invalidate 本 key。 */
export function useProviderModelCatalog() {
  return useQuery({
    queryKey: providerKeys.catalog(),
    queryFn: getProviderModelCatalog,
  });
}

export function useSettings() {
  return useQuery<Settings>({
    queryKey: settingsKeys.all,
    queryFn: getSettings,
    staleTime: 5 * 60_000,
  });
}

export function useCreateProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: createProvider,
    // 后端 POST /providers 会 TriggerAutoAssociate，可能新增关联行
    onSuccess: () => {
      invalidateAfterProviderWrite(qc);
    },
  });
}

export function useUpdateProvider(id: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (data: Parameters<typeof updateProvider>[1]) =>
      updateProvider(id, data),
    // 同 POST：PUT 会 TriggerAutoAssociate + TriggerAutoClean
    onSuccess: () => {
      invalidateAfterProviderWrite(qc);
    },
  });
}

export function useDeleteProvider() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: deleteProvider,
    // 乐观更新：立即从缓存中移除被删项，让 AnimatePresence 稳定接管退场动画
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: providerKeys.lists() });
      const previous = qc.getQueriesData<Provider[]>({ queryKey: providerKeys.lists() });
      qc.setQueriesData<Provider[]>(
        { queryKey: providerKeys.lists() },
        (old) => old?.filter((p) => p.ID !== id),
      );
      return { previous };
    },
    onError: (_err, _id, context) => {
      // 请求失败回滚到乐观更新前的快照
      if (context?.previous) {
        for (const [key, data] of context.previous) {
          qc.setQueryData(key, data);
        }
      }
    },
    onSettled: () => {
      // 删除提供商后其名下关联随之消失，两个缓存族都要刷
      invalidateAfterProviderWrite(qc);
    },
  });
}

export function useClearProviderAssociations() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: clearProviderAssociations,
    // 清除的是 model-providers 表的行：提供商页读 providerKeys，模型关联页读
    // modelProviderKeys（另一套 key），两族必须同时失效——见 invalidateAfterProviderWrite。
    onSettled: () => {
      invalidateAfterProviderWrite(qc);
    },
  });
}

export function useSyncAllProviders() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: syncAllProviderModels,
    onSuccess: () => {
      // 同步改写分组白名单（→ 组织级目录变化），并在开关开启时经
      // modelsync.ActionHooks 后台改写关联，故三个缓存族一起失效。
      invalidateAfterProviderWrite(qc, { catalog: true });
    },
  });
}

export function useHealthCheckSettingsQuery() {
  return useQuery<HealthCheckSettings>({
    queryKey: healthCheckSettingsKeys.all,
    queryFn: getHealthCheckSettings,
    staleTime: 5 * 60_000,
  });
}

export function useUpdateSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: settingsKeys.all });
    },
  });
}

export function useUpdateHealthCheckSettingsMutation() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: updateHealthCheckSettings,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: healthCheckSettingsKeys.all });
    },
  });
}

export type { Provider, ProviderModelCatalog, ProviderTemplate, Settings, HealthCheckSettings };
