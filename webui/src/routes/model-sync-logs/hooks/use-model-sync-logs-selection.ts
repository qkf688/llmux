import { useCallback, useEffect, useMemo } from "react";
import type { ModelSyncLog } from "@/lib/api";

type Updater<T> = T | ((previous: T) => T);

type UseModelSyncLogsSelectionInput = {
  activeTab: string;
  logs: ModelSyncLog[];
  recentErrors: ModelSyncLog[];
  selectedLogs: Set<number>;
  selectedErrorProviders: Set<number>;
  page: number;
  totalPages: number;
  showUnchanged: boolean;
  setSelectedLogs: (next: Updater<Set<number>>) => void;
  setSelectedErrorProviders: (next: Updater<Set<number>>) => void;
  setPage: (page: number) => void;
  setShowUnchanged: (show: boolean) => void;
};

export function useModelSyncLogsSelection({
  activeTab,
  logs,
  recentErrors,
  selectedLogs,
  selectedErrorProviders,
  page,
  totalPages,
  showUnchanged,
  setSelectedLogs,
  setSelectedErrorProviders,
  setPage,
  setShowUnchanged,
}: UseModelSyncLogsSelectionInput) {
  useEffect(() => {
    setSelectedLogs(new Set());
  }, [activeTab, page, setSelectedLogs, showUnchanged]);

  useEffect(() => {
    if (activeTab !== "errors") {
      setSelectedErrorProviders(new Set());
    }
  }, [activeTab, setSelectedErrorProviders]);

  const selectedCount = selectedLogs.size;
  const allSelected = logs.length > 0 && logs.every((log) => selectedLogs.has(log.ID));
  const canDeleteSelected = selectedCount > 0;

  const recentErrorProviderIds = useMemo(
    () => Array.from(new Set(recentErrors.map((log) => log.ProviderID))),
    [recentErrors]
  );

  const selectedErrorProvidersCount = selectedErrorProviders.size;
  const allErrorProvidersSelected =
    recentErrorProviderIds.length > 0 && recentErrorProviderIds.every((id) => selectedErrorProviders.has(id));

  useEffect(() => {
    if (selectedErrorProviders.size === 0) {
      return;
    }

    const allowed = new Set(recentErrorProviderIds);
    setSelectedErrorProviders((previous) => {
      if (previous.size === 0) {
        return previous;
      }

      const next = new Set<number>();
      for (const providerId of previous) {
        if (allowed.has(providerId)) {
          next.add(providerId);
        }
      }

      return next.size === previous.size ? previous : next;
    });
  }, [recentErrorProviderIds, selectedErrorProviders.size, setSelectedErrorProviders]);

  const handleToggleSelectAll = () => {
    if (allSelected) {
      setSelectedLogs(new Set());
      return;
    }

    setSelectedLogs(new Set(logs.map((log) => log.ID)));
  };

  const handleToggleSelectLog = (logId: number, checked: boolean) => {
    setSelectedLogs((previous) => {
      const next = new Set(previous);
      if (checked) {
        next.add(logId);
      } else {
        next.delete(logId);
      }
      return next;
    });
  };

  const handlePageChange = (newPage: number) => {
    if (newPage < 1 || newPage > totalPages || newPage === page) {
      return;
    }
    setPage(newPage);
  };

  const handleShowUnchangedChange = (checked: boolean) => {
    setShowUnchanged(checked);
  };

  const handleToggleSelectAllErrorProviders = () => {
    if (allErrorProvidersSelected) {
      setSelectedErrorProviders(new Set());
      return;
    }
    setSelectedErrorProviders(new Set(recentErrorProviderIds));
  };

  const handleToggleSelectErrorProvider = (providerId: number, checked: boolean) => {
    setSelectedErrorProviders((previous) => {
      const next = new Set(previous);
      if (checked) {
        next.add(providerId);
      } else {
        next.delete(providerId);
      }
      return next;
    });
  };

  const isLogSelected = useCallback((logId: number) => selectedLogs.has(logId), [selectedLogs]);
  const isErrorProviderSelected = useCallback(
    (providerId: number) => selectedErrorProviders.has(providerId),
    [selectedErrorProviders]
  );

  return {
    selectedCount,
    allSelected,
    canDeleteSelected,
    selectedErrorProvidersCount,
    allErrorProvidersSelected,
    recentErrorProviderIds,
    handleToggleSelectAll,
    handleToggleSelectLog,
    handlePageChange,
    handleShowUnchangedChange,
    handleToggleSelectAllErrorProviders,
    handleToggleSelectErrorProvider,
    isLogSelected,
    isErrorProviderSelected,
  };
}
