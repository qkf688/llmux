export function hasActiveProvidersFilter(nameFilter: string, typeFilter: string): boolean {
  return nameFilter.trim() !== "" || typeFilter !== "all";
}

