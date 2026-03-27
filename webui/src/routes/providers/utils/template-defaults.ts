import type { UseFormReturn } from "react-hook-form";
import type { ProviderTemplate } from "@/lib/api";
import type { ProviderFormValues } from "../form-schema";
import { parseConfigToForm } from "./config";

export function applyProviderTemplateDefaults(
  type: string,
  providerTemplates: ProviderTemplate[],
  form: UseFormReturn<ProviderFormValues>,
): void {
  const selectedTemplate = providerTemplates.find((template) => template.type === type);
  if (!selectedTemplate) return;

  const parsed = parseConfigToForm(selectedTemplate.template);

  const currentBaseUrl = form.getValues("base_url");
  if (!currentBaseUrl || currentBaseUrl.trim() === "") {
    form.setValue("base_url", parsed.base_url);
  }

  if (type === "anthropic") {
    form.setValue("version", parsed.version || "2023-06-01");
    form.setValue("beta", parsed.beta || "");
  }
}

