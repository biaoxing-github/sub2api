package handler

// openAIWSNextAttemptMessage 选择当前轮的完整副本，拒绝以首包替代缺失的后续轮上下文。
func openAIWSNextAttemptMessage(current, retryPayload []byte, retryCurrentTurn bool) ([]byte, bool) {
	if retryCurrentTurn {
		if len(retryPayload) == 0 {
			return nil, false
		}
		return append([]byte(nil), retryPayload...), true
	}
	return append([]byte(nil), current...), true
}
