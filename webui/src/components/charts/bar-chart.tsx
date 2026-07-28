"use client"

import { Bar, BarChart, CartesianGrid, LabelList, XAxis, YAxis, type LabelProps } from "recharts"

import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from "@/components/ui/chart"
import type { ModelCount } from "@/lib/api"

// 预定义颜色数组，按顺序生成颜色
const predefinedColors = [
  "var(--chart-1)",
  "var(--chart-2)",
  "var(--chart-3)",
  "var(--chart-4)",
  "var(--chart-5)",
  "var(--chart-6)",
  "var(--chart-7)",
  "var(--chart-8)",
  "var(--chart-9)",
  "var(--chart-10)",
]

// 根据模型数据生成图表配置
const generateChartConfig = (data: ModelCount[]) => {
  const config: ChartConfig = {
    calls: {
      label: "调用次数",
    },
  }

  data.forEach((item, index) => {
    config[item.model] = {
      label: item.model,
      color: predefinedColors[index % predefinedColors.length],
    }
  })

  return config
}

// 根据模型数据生成图表数据
const generateChartData = (data: ModelCount[]) => {
  return data.map((item, index) => ({
    model: item.model,
    calls: item.calls,
    fill: predefinedColors[index % predefinedColors.length],
  }))
}

interface ModelRankingChartProps {
  data: ModelCount[]
}

export function ModelRankingChart({ data }: ModelRankingChartProps) {
  const chartData = generateChartData(data)
  const chartConfig = generateChartConfig(data)

  const renderModelLabel = (props: LabelProps) => {
    const { x, y, width, height, value } = props
    if (typeof value !== "string") return null
    if (typeof x !== "number" || typeof y !== "number" || typeof width !== "number" || typeof height !== "number") return null
    if (width <= 0 || height <= 0) return null

    return (
      <foreignObject x={x} y={y} width={width} height={height}>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            height: "100%",
            paddingLeft: 8,
            paddingRight: 6,
            overflow: "hidden",
          }}
        >
          <span
            style={{
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
              color: "white",
              fontWeight: 500,
              fontSize: 12,
              lineHeight: "14px",
            }}
            title={value}
          >
            {value}
          </span>
        </div>
      </foreignObject>
    )
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>模型调用排行</CardTitle>
      </CardHeader>
      <CardContent>
        <ChartContainer config={chartConfig} className="max-h-[500px] sm:max-h-[390px]">
          <BarChart
            accessibilityLayer
            data={chartData}
            layout="vertical"
            margin={{
              right: 16,
            }}
          >
            <CartesianGrid horizontal={false} />
            <YAxis
              dataKey="model"
              type="category"
              tickLine={false}
              tickMargin={10}
              axisLine={false}
              tickFormatter={(value) => value.slice(0, 10)}
              hide
            />
            <XAxis dataKey="calls" type="number" hide />
            <ChartTooltip
              cursor={false}
              content={<ChartTooltipContent indicator="line" hideLabel />}
            />
            <Bar
              dataKey="calls"
              layout="vertical"
              fill="var(--color-calls)"
              radius={4}
            >
              <LabelList
                dataKey="model"
                content={renderModelLabel}
              />
              <LabelList
                dataKey="calls"
                position="right"
                offset={8}
                className="fill-foreground"
                fontSize={12}
              />
            </Bar>
          </BarChart>
        </ChartContainer>
      </CardContent>
    </Card>
  )
} 
