export function formatSyncDate(dateStr?: string) {
  if (!dateStr) {
    return "-";
  }
  return new Date(dateStr).toLocaleString("zh-CN");
}
