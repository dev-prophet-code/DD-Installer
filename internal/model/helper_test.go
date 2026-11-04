package model

import "encoding/json"

func mustMarshal(v any) string { b, _ := json.Marshal(v); return string(b) }
