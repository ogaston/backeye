package main

import (
	"encoding/json"
	"time"
)

type DetectionAlert struct {
	NodeID    string    `json:"node_id"`
	Location  string    `json:"location"`
	FaceCount int       `json:"face_count"`
	Timestamp time.Time `json:"timestamp"`
}

func MarshalAlert(a DetectionAlert) ([]byte, error) {
	return json.Marshal(a)
}

func UnmarshalAlert(data []byte) (DetectionAlert, error) {
	var a DetectionAlert
	err := json.Unmarshal(data, &a)
	return a, err
}
