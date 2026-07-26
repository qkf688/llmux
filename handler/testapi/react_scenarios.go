package testapi

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
)

type reactScenario struct {
	Question       string
	ExpectedCities []string
}

var reactScenarios = []reactScenario{
	{
		Question:       "分两次获取一下南京和北京的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"南京", "北京"},
	},
	{
		Question:       "分两次获取一下上海和广州的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"上海", "广州"},
	},
	{
		Question:       "分两次获取一下深圳和杭州的天气 每次调用后回复我对应城市的总结信息",
		ExpectedCities: []string{"深圳", "杭州"},
	},
}

func selectScenarioIndex(modelID, rawIndex string, scenarioCount int) (int, error) {
	if scenarioCount <= 0 {
		return 0, errors.New("no react scenarios configured")
	}
	if strings.TrimSpace(rawIndex) == "" {
		return stableIndex(modelID, scenarioCount), nil
	}
	parsed, err := strconv.Atoi(rawIndex)
	if err != nil {
		return 0, fmt.Errorf("invalid scenario index: %s", rawIndex)
	}
	if parsed < 0 || parsed >= scenarioCount {
		return 0, fmt.Errorf("scenario index out of range: %d", parsed)
	}
	return parsed, nil
}

func stableIndex(key string, mod int) int {
	if mod <= 0 {
		return 0
	}
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(key))
	return int(hash.Sum32() % uint32(mod))
}
