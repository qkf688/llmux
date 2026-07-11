import type { UseFormReturn } from "react-hook-form";
import type { ProviderTemplate } from "@/lib/api";
import type { ProviderFormValues } from "../form-schema";
import { getExtraFieldDefaultsFromParsed } from "../form-fields";
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

  const extras = getExtraFieldDefaultsFromParsed(type, {
    version: parsed.version,
    beta: parsed.beta,
    auth_type: parsed.auth_type,
  });

  for (const [name, value] of Object.entries(extras)) {
    form.setValue(name as keyof ProviderFormValues, value);
  }
}