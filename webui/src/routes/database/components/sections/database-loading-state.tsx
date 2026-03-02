import { Card, CardContent } from "@/components/ui/card";
import { RefreshCw } from "lucide-react";

export function DatabaseLoadingState() {
  return (
    <Card>
      <CardContent className="flex items-center justify-center py-12">
        <div className="text-center">
          <RefreshCw className="h-8 w-8 animate-spin mx-auto mb-4 text-muted-foreground" />
          <p className="text-muted-foreground">加载中...</p>
        </div>
      </CardContent>
    </Card>
  );
}
