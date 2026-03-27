import { useEffect, useMemo } from "react";
import { toast } from "sonner";
import { getProviderBlacklist, updateProviderBlacklist, type Provider } from "@/lib/api";
import type { BlacklistFilter } from "../types";

type Updater<T> = T | ((previous: T) => T);
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
