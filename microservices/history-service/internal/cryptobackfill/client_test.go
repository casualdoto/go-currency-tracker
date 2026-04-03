package cryptobackfill

import (
	"encoding/json"
	"testing"
)

func TestParseKlineRow(t *testing.T) {
	raw := []byte(`[1499040000000,"0.01634790","0.80000000","0.01575800","0.01577100","148976.11427815"]`)
	var row []interface{}
	if err := json.Unmarshal(raw, &row); err != nil {
		t.Fatal(err)
	}
	k, ok := parseKlineRow(row)
	if !ok {
		t.Fatal("expected ok")
	}
	if k.openTimeMs != 1499040000000 {
		t.Fatalf("openTimeMs: got %d", k.openTimeMs)
	}
	if k.close < 0.015 || k.close > 0.016 {
		t.Fatalf("close: got %v", k.close)
	}
}
