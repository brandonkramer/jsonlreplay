package jsonlreplay

import "encoding/json"

//
// ────────────────────────────────────────
// seq helpers.
//

// seqFromLine reads the top-level "seq" field when present and positive.
func seqFromLine(line []byte) (seq int64, ok bool, err error) {
	var head struct {
		Seq int64 `json:"seq"`
	}
	if err := json.Unmarshal(line, &head); err != nil {
		return 0, false, err
	}
	if head.Seq <= 0 {
		return 0, false, nil
	}
	return head.Seq, true, nil
}

// injectSeq merges seq into a JSON object line. Other fields are preserved.
func injectSeq(line []byte, seq int64, timeStr string, addTime bool) ([]byte, error) {
	m := map[string]json.RawMessage{}
	if len(line) > 0 {
		if err := json.Unmarshal(line, &m); err != nil {
			return nil, err
		}
	}
	seqJSON, err := json.Marshal(seq)
	if err != nil {
		return nil, err
	}
	m["seq"] = seqJSON
	if addTime && timeStr != "" {
		if _, has := m["time"]; !has {
			timeJSON, err := json.Marshal(timeStr)
			if err != nil {
				return nil, err
			}
			m["time"] = timeJSON
		}
	}
	return json.Marshal(m)
}
