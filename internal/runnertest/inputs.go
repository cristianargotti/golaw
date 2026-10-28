package runnertest

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// consume does what a behavior asks before the answer: it records the
// bytes of stdin when asked, so a test proves the hook piped the file and
// wrote no temporary copy, and it waits the delay, so a test proves the
// kill of a slow tool.
func (b Behavior) consume(base string, stdin io.Reader) error {
	var delay time.Duration
	if b.Delay != "" {
		parsed, err := time.ParseDuration(b.Delay)
		if err != nil {
			return fmt.Errorf("parse the delay: %w", err)
		}
		delay = parsed
	}
	if b.Stdin {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("read stdin: %w", err)
		}
		if err := appendLine(base+inputsSuffix, data); err != nil {
			return err
		}
	}
	time.Sleep(delay)
	return nil
}

// appendLine appends one JSON value per call, as record does for the
// arguments, because each call is a process of its own.
func appendLine(path string, value []byte) error {
	line, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode the input: %w", err)
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, fileMode) //nolint:gosec // the record sits beside the fake its test installed
	if err != nil {
		return fmt.Errorf("open the input record: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("write the input record: %w", err)
	}
	return nil
}

// Inputs answers the stdin of every recorded call of the fake tool name in
// dir, in order, none for a tool never started or whose behavior did not
// ask for its stdin.
func Inputs(tb testing.TB, dir, name string) [][]byte {
	tb.Helper()
	data, err := os.ReadFile(filepath.Join(dir, name) + inputsSuffix)
	if errors.Is(err, fs.ErrNotExist) {
		return [][]byte{}
	}
	if err != nil {
		tb.Fatalf("read the inputs of %s: %v", name, err)
	}
	inputs := [][]byte{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	for {
		var input []byte
		switch err := decoder.Decode(&input); {
		case errors.Is(err, io.EOF):
			return inputs
		case err != nil:
			tb.Fatalf("decode an input of %s: %v", name, err)
			return inputs
		}
		if input == nil {
			input = []byte{}
		}
		inputs = append(inputs, input)
	}
}
