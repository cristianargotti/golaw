package cursor

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cristianargotti/golaw/internal/hook"
	"github.com/cristianargotti/golaw/internal/model"
)

// notBuilt is the cause of every event until the adapter is built: the
// hook fails closed meanwhile.
func notBuilt() error {
	return fmt.Errorf("%w: the cursor adapter is not built yet", model.ErrCannotJudge)
}

// denial is the answer of a pre event the hook refuses.
type denial struct {
	Permission   string `json:"permission"`
	UserMessage  string `json:"user_message"`
	AgentMessage string `json:"agent_message"`
}

// Parse decodes an event once: the envelope, the header enrollment reads
// and the decoder of its request.
func Parse([]byte) (hook.Parsed, error) {
	return hook.Parsed{}, notBuilt()
}

// Oversized parses an event over the bound of the hook in bounded memory.
func Oversized(io.Reader) (hook.Parsed, error) {
	return hook.Parsed{}, notBuilt()
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
