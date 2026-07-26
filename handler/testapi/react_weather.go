package testapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v2"
	"github.com/tidwall/gjson"
)

type weatherStub struct {
	Desc string
	Temp int // Celsius
}

var weatherStubs = map[string]weatherStub{
	"南京": {Desc: "晴转多云", Temp: 18},
	"北京": {Desc: "大雨转小雨", Temp: 15},
	"上海": {Desc: "多云", Temp: 22},
	"广州": {Desc: "雷阵雨", Temp: 27},
	"深圳": {Desc: "小雨", Temp: 26},
	"杭州": {Desc: "阴", Temp: 20},
}

func GetWeather(ctx context.Context, call openai.ChatCompletionChunkChoiceDeltaToolCallFunction) (*openai.ChatCompletionToolMessageParamContentUnion, error) {
	if call.Name != "get_weather" {
		return nil, fmt.Errorf("invalid tool call name: %s", call.Name)
	}

	_, res := weatherFromArguments(call.Arguments)

	return &openai.ChatCompletionToolMessageParamContentUnion{
		OfString: openai.String(res),
	}, nil
}

func weatherFromArguments(arguments string) (city string, result string) {
	location := gjson.Get(arguments, "location").String()
	unit := gjson.Get(arguments, "unit").String()
	city = normalizeCity(location)
	res, ok := buildWeatherStub(city, unit)
	if !ok {
		res = "暂不支持该地区天气查询"
	}
	return city, res
}

func buildWeatherToolSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]string{
				"type":        "string",
				"description": "The city name",
			},
			"unit": map[string]any{
				"type":        "string",
				"description": "Temperature unit (celsius/fahrenheit)",
				"enum":        []string{"celsius", "fahrenheit"},
			},
		},
		"required": []string{"location"},
	}
}

func buildWeatherStub(city, unit string) (string, bool) {
	if strings.TrimSpace(city) == "" {
		return "", false
	}
	stub, ok := weatherStubs[city]
	if !ok {
		return "", false
	}

	switch strings.ToLower(strings.TrimSpace(unit)) {
	case "", "celsius":
		return fmt.Sprintf("%s天气%s，温度 %d℃", city, stub.Desc, stub.Temp), true
	case "fahrenheit":
		tempF := int(float64(stub.Temp)*9.0/5.0 + 32.0)
		return fmt.Sprintf("%s天气%s，温度 %d°F", city, stub.Desc, tempF), true
	default:
		return fmt.Sprintf("%s天气%s，温度 %d℃", city, stub.Desc, stub.Temp), true
	}
}
