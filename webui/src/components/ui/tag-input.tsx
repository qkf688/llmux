import * as React from "react"
import { X, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { cn } from "@/lib/utils"

export interface TagInputProps {
  value: string[]
  onChange: (tags: string[]) => void
  placeholder?: string
  className?: string
}

export function TagInput({ value, onChange, placeholder, className }: TagInputProps) {
  const [inputValue, setInputValue] = React.useState("")

  const handleAdd = () => {
    const trimmed = inputValue.trim()
    if (trimmed && !value.includes(trimmed)) {
      onChange([...value, trimmed])
    }
    setInputValue("")
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault()
      handleAdd()
    }
  }

  const removeTag = (tagToRemove: string) => {
    onChange(value.filter(tag => tag !== tagToRemove))
  }

  return (
    <div className={cn("space-y-2", className)}>
      {/* 输入区域 */}
      <div className="flex gap-2">
        <Input
          type="text"
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          className="flex-1"
        />
        <Button
          type="button"
          onClick={handleAdd}
          size="default"
          className="shrink-0"
        >
          <Plus className="h-4 w-4 mr-1" />
          添加
        </Button>
      </div>

      {/* 列表区域 */}
      <div className="min-h-[100px] rounded-md border border-input bg-transparent p-3">
        {value.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            暂无内容，请在上方输入添加
          </p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {value.map((tag) => (
              <div
                key={tag}
                className="flex items-center gap-1.5 rounded-md bg-secondary px-2.5 py-1 text-sm"
              >
                <span>{tag}</span>
                <button
                  type="button"
                  onClick={() => removeTag(tag)}
                  className="rounded-sm text-muted-foreground hover:text-destructive focus:outline-none focus:ring-1 focus:ring-ring transition-colors"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
