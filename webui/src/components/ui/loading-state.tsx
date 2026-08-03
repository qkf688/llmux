import { Spinner } from "@/components/ui/spinner"
import { cn } from "@/lib/utils"

interface LoadingStateProps {
  /** 加载文案，默认 "加载中..." */
  text?: string
  /** 外层容器 className */
  className?: string
  /** Spinner className（默认 h-8 w-8） */
  spinnerClassName?: string
}

/** 居中加载态：Spinner + 文案组合。复用 Spinner，统一项目加载视觉。 */
function LoadingState({ text = "加载中...", className, spinnerClassName }: LoadingStateProps) {
  return (
    <div className={cn("flex items-center justify-center py-4", className)}>
      <Spinner className={cn("h-8 w-8", spinnerClassName)} />
      {text && <span className="ml-2">{text}</span>}
    </div>
  )
}

export { LoadingState }
