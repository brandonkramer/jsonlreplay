package jsonlreplay

import "encoding/json"

//
// ────────────────────────────────────────
// codec.
//

// Codec marshals and unmarshals log lines. Nil Options.Codec uses EventCodec.
type Codec interface {
	Marshal(v any) ([]byte, error)
	Unmarshal(line []byte, v any) error
}

// EventCodec is the default codec for Event and any JSON-marshalable value.
type EventCodec struct{}

// Marshal encodes v as JSON. Event values use the standard event shape.
func (EventCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Unmarshal decodes one JSON line into v.
func (EventCodec) Unmarshal(line []byte, v any) error {
	return json.Unmarshal(line, v)
}

func (o Options) codec() Codec {
	if o.Codec != nil {
		return o.Codec
	}
	return EventCodec{}
}
