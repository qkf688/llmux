import { useEffect, useMemo } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { getProviderBlacklist, updateProviderBlacklist, type Provider } from "@/lib/api";
import { providerKeys } from "@/hooks/api/use-providers";
import type { Updater } from "@/stores/core/updater";
import type { BlacklistFilter } from "../types";

type Setter<T> = (value: Updater<T>) => void;

type UseModelProvidersBlacklistInput = {
  providers: Provider[];

  blacklistDialogOpen: boolean;
  setBlacklistDialogOpen: (open: boolean) => void;

  blacklistedIds: number[];
  setBlacklistedIds: Setter<number[]>;

  setBlacklistLoading: (loading: boolean) => void;
  setBlacklistSaving: (saving: boolean) => void;

  blacklistSearchTerm: string;
  setBlacklistSearchTerm: (term: string) => void;
  blacklistFilter: BlacklistFilter;
  setBlacklistFilter: (filter: BlacklistFilter) => void;
};

export function useModelProvidersBlacklist({
  providers,
  blacklistDialogOpen,
  setBlacklistDialogOpen,
  blacklistedIds,
  setBlacklistedIds,
  setBlacklistLoading,
  setBlacklistSaving,
  blacklistSearchTerm,
  setBlacklistSearchTerm,
  blacklistFilter,
  setBlacklistFilter,
}: UseModelProvidersBlacklistInput) {
  const queryClient = useQueryClient();

  const filteredProviders = useMemo(() => {
    let result = providers;

    if (blacklistFilter === "blacklisted") {
      result = result.filter((provider) => blacklistedIds.includes(provider.ID));
    } else if (blacklistFilter === "not-blacklisted") {
      result = result.filter((provider) => !blacklistedIds.includes(provider.ID));
    }

    if (blacklistSearchTerm.trim()) {
      const term = blacklistSearchTerm.toLowerCase().trim();
      result = result.filter(
        (provider) =>
          provider.Name.toLowerCase().includes(term) || provider.Type.toLowerCase().includes(term)
      );
    }

    return result;
  }, [blacklistFilter, blacklistedIds, blacklistSearchTerm, providers]);

  useEffect(() => {
    if (!blacklistDialogOpen) return;
    setBlacklistLoading(true);
    getProviderBlacklist()
      .then((data) => setBlacklistedIds(data.blacklisted_ids))
      .catch((err) => toast.error(`加载黑名单失败: ${err instanceof Error ? err.message : String(err)}`))
      .finally(() => setBlacklistLoading(false));
  }, [blacklistDialogOpen, setBlacklistLoading, setBlacklistedIds]);

  const handleSaveBlacklist = async () => {
    setBlacklistSaving(true);
    try {
      await updateProviderBlacklist(blacklistedIds);
      // 黑名单写的是 Provider.Blacklisted，提供商页的「关联触发」开关直接读该字段。
      // 不失效 providerKeys 的话，回到提供商页仍显示旧开关状态（staleTime 60s）。
      // 该端点（PUT /providers/blacklist）本身不触发 autoassoc，故关联行不变，
      // modelProviderKeys 无需失效。
      void queryClient.invalidateQueries({ queryKey: providerKeys.lists() });
      toast.success("黑名单已保存");
      setBlacklistDialogOpen(false);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`保存黑名单失败: ${message}`);
    } finally {
      setBlacklistSaving(false);
    }
  };

  const handleToggleBlacklist = (providerId: number, checked: boolean) => {
    setBlacklistedIds((prev) => (checked ? [...prev, providerId] : prev.filter((id) => id !== providerId)));
  };

  const openBlacklistDialog = () => {
    setBlacklistDialogOpen(true);
  };

  const cancelBlacklistDialog = () => {
    setBlacklistDialogOpen(false);
    setBlacklistSearchTerm("");
    setBlacklistFilter("all");
  };

  return {
    filteredProviders,
    openBlacklistDialog,
    cancelBlacklistDialog,
    handleSaveBlacklist,
    handleToggleBlacklist,
  };
}
