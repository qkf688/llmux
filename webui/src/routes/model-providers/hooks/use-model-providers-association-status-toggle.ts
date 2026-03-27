import { useCallback } from "react";
import { updateModelProviderStatus, type ModelWithProvider } from "@/lib/api";

type Setter<T> = (value: T | ((previous: T) => T)) => void;

type UseModelProvidersAssociationStatusToggleInput = {
  setModelProviders: Setter<ModelWithProvider[]>;
  setStatusUpdating: Setter<Record<number, boolean>>;
  setStatusError: Setter<string | null>;
};

export function useModelProvidersAssociationStatusToggle({
  setModelProviders,
  setStatusUpdating,
  setStatusError,
}: UseModelProvidersAssociationStatusToggleInput) {
  const handleStatusToggle = useCallback(
    async (association: ModelWithProvider, nextStatus: boolean) => {
      const previousStatus = association.Status ?? true;
      setStatusError(null);
      setStatusUpdating((prev) => ({ ...prev, [association.ID]: true }));
      setModelProviders((prev) => prev.map((item) => (item.ID === association.ID ? { ...item, Status: nextStatus } : item)));

      try {
        const updated = await updateModelProviderStatus(association.ID, nextStatus);
        const normalized = { ...updated, CustomerHeaders: updated.CustomerHeaders || {} };
        setModelProviders((prev) => prev.map((item) => (item.ID === association.ID ? normalized : item)));
      } catch (err) {
        setModelProviders((prev) =>
          prev.map((item) => (item.ID === association.ID ? { ...item, Status: previousStatus } : item))
        );
        setStatusError("更新启用状态失败");
        console.error(err);
      } finally {
        setStatusUpdating((prev) => {
          const next = { ...prev };
          delete next[association.ID];
          return next;
        });
      }
    },
    [setModelProviders, setStatusError, setStatusUpdating]
  );

  return { handleStatusToggle };
}

