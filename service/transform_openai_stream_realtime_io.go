package service

import (
	"encoding/json"
	"fmt"
)

func writeRealtimeJSONData(state *realtimeStreamState, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return writeRealtimeData(state, string(data))
}

func writeRealtimeEventJSONData(state *realtimeStreamState, event string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(state.writer, "event: %s\ndata: %s\n\n", event, string(data))
	return err
}

func writeRealtimeOrderedData(state *realtimeStreamState, payload map[string]interface{}) error {
	data, err := marshalWithTypeFirst(payload)
	if err != nil {
		return err
	}
	return writeRealtimeData(state, string(data))
}
