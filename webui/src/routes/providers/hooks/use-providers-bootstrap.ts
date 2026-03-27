import { useCallback, useEffect } from "react";
import { toast } from "sonner";
import { getProviderTemplates, getProviders, getSettings, type Provider, type ProviderTemplate } from "@/lib/api";

type Updater<T> = T | ((previous: T) => T);
type Setter<T> = (value: Updater<T>) => void;

type UseProvidersBootstrapInput = {
  debouncedNameFilter: string;
  typeFilter: string;

  setLoading: (loading: boolean) => void;
  setProviders: Setter<Provider[]>;

  setProviderTemplates: Setter<ProviderTemplate[]>;
  setAvailableTypes: (types: string[]) => void;

  setAutoAssociateOnAddEnabled: (enabled: boolean) => void;
  setAutoCleanOnDeleteEnabled: (enabled: boolean) => void;
};

export function useProvidersBootstrap({
  debouncedNameFilter,
  typeFilter,
  setLoading,
  setProviders,
  setProviderTemplates,
  setAvailableTypes,
  setAutoAssociateOnAddEnabled,
  setAutoCleanOnDeleteEnabled,
}: UseProvidersBootstrapInput) {
  const fetchProviders = useCallback(async () => {
    try {
      setLoading(true);
      const name = debouncedNameFilter.trim() || undefined;
      const type = typeFilter === "all" ? undefined : typeFilter;

      const data = await getProviders({ name, type });
      setProviders(data);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      toast.error(`获取提供商列表失败: ${message}`);
      console.error(err);
    } finally {
      setLoading(false);
    }
  }, [debouncedNameFilter, setLoading, setProviders, typeFilter]);

  const fetchProviderTemplates = useCallback(async () => {
    try {
      const data = await getProviderTemplates();
      setProviderTemplates(data);
      const types = data.map((template) => template.type);
      setAvailableTypes(types);
    } catch (err) {
      console.error("获取提供商模板失败", err);
    }
  }, [setAvailableTypes, setProviderTemplates]);

  const fetchSettings = useCallback(async () => {
    try {
      const data = await getSettings();
      setAutoAssociateOnAddEnabled(data.auto_associate_on_add ?? false);
      setAutoCleanOnDeleteEnabled(data.auto_clean_on_delete ?? false);
    } catch (err) {
      console.error("获取系统设置失败", err);
      setAutoAssociateOnAddEnabled(false);
      setAutoCleanOnDeleteEnabled(false);
    }
  }, [setAutoAssociateOnAddEnabled, setAutoCleanOnDeleteEnabled]);

  useEffect(() => {
    void fetchProviderTemplates();
    void fetchSettings();
  }, [fetchProviderTemplates, fetchSettings]);

  useEffect(() => {
    void fetchProviders();
  }, [fetchProviders]);

  return { fetchProviders };
}

