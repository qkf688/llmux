import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import type { ExportType } from "@/lib/api";
import { EXPORT_TYPE_OPTIONS } from "../../types";

type ExportTypeCheckboxGroupProps = {
  prefix: "export" | "import";
  selectedTypes: ExportType[];
  onToggleType: (type: ExportType) => void;
};

export function ExportTypeCheckboxGroup({
  prefix,
  selectedTypes,
  onToggleType,
}: ExportTypeCheckboxGroupProps) {
  return (
    <div className="space-y-3">
      {EXPORT_TYPE_OPTIONS.map((option) => {
        const inputId = `${prefix}-${option.type}`;
        const checked = selectedTypes.includes(option.type);

        return (
          <div className="flex items-center space-x-2" key={option.type}>
            <Checkbox
              id={inputId}
              checked={checked}
              onCheckedChange={() => onToggleType(option.type)}
            />
            <Label htmlFor={inputId} className="cursor-pointer">
              {option.label}
            </Label>
          </div>
        );
      })}
    </div>
  );
}
