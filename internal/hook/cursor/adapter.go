package cursor

import (
	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// Parse decodes an event once: the envelope, the header enrollment reads
// and the decoder of its request, which reads the fields the envelope
// kept and none of the bytes again.
func Parse(data []byte) (hook.Parsed, error) {
	input, err := parsePayload(data)
	if err != nil {
		return hook.Parsed{}, err
	}
	event, err := eventHeader(input)
	if err != nil {
		return hook.Parsed{}, err
	}
	event = withReadTarget(input, event)
	return hook.Parsed{Event: event, Decode: func(base hook.Base, snapshot model.Snapshot) (hook.Request, error) {
		return decodeRequest(input, event, base, snapshot)
	}}, nil
}
