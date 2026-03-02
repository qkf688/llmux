import { Card, CardContent } from "@/components/ui/card";

export function DatabaseErrorState() {
  return (
    <Card>
      <CardContent className="flex items-center justify-center py-12">
        <p className="text-muted-foreground">无法加载数据库信息</p>
      </CardContent>
    </Card>
  );
}
