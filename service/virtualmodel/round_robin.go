package virtualmodel

import "log/slog"

// UpdateRoundRobinIndex 更新轮询索引（在请求成功后调用）。
// candidateCount 必须是经过 loadCandidatePool 过滤后的候选池长度，
// 不能用全量启用映射数，否则黑名单/缺失模型会导致取模基数与选路时不一致。
func (s *Service) UpdateRoundRobinIndex(virtualModelID uint, candidateCount int) {
	if candidateCount <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentIndex := s.roundRobinState[virtualModelID]
	s.roundRobinState[virtualModelID] = (currentIndex + 1) % candidateCount

	slog.Debug("updated round_robin index", "virtual_model_id", virtualModelID, "new_index", s.roundRobinState[virtualModelID])
}
