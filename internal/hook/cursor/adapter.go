package cursor

import (
	"encoding/json"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// denial is the answer of a pre event the hook refuses.
type denial struct {
	Permission   string `json:"permission"`
	UserMessage  string `json:"user_message"`
	AgentMessage string `json:"agent_message"`
}

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

// Encode answers a result in the shape the host reads: until the adapter
// is built, every result leaves as the denial JSON with exit 0, the agent
// reading the English reason and the person the human message.
func Encode(_ model.Event, result hook.Result) hook.Reply {
	data, err := json.Marshal(denial{Permission: "deny", UserMessage: result.Human, AgentMessage: result.Reason})
	if err != nil {
		return hook.Reply{ExitCode: 2, Stderr: "cannot judge: response encoding failed\n"}
	}
	return hook.Reply{Stdout: append(data, '\n')}
}
