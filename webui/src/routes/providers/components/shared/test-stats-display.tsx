interface TestStatsDisplayProps {
  tested: number;
  success: number;
  failed: number;
}

export function TestStatsDisplay({
  tested,
  success,
  failed,
}: TestStatsDisplayProps) {
  return (
    <div className="flex items-center gap-2 text-xs text-muted-foreground flex-shrink-0">
      <span>已测试: {tested}</span>
      <span className="text-muted-foreground">|</span>
      <span className="text-green-600">成功: {success}</span>
      <span className="text-muted-foreground">|</span>
      <span className="text-red-600">失败: {failed}</span>
    </div>
  );
}
