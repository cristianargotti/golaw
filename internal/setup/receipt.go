package setup

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
)

// ReceiptSchema prevents an older binary from interpreting newer ownership data.
const ReceiptSchema = 1

// Receipt binds the armed hooks and tool paths to one repository identity.
type Receipt struct {
	Schema    int                `json:"schema"`
	Identity  string             `json:"identity"`
	Root      string             `json:"root"`
	Version   string             `json:"version"`
	Tools     []ToolRecord       `json:"tools"`
	Hooks     []install.Artifact `json:"hooks"`
	HooksPath string             `json:"hooks_path"`
	Golaw     string             `json:"golaw"`
	ArmedAt   time.Time          `json:"armed_at"`
}

// ToolRecord preserves the executable setup verified for later diagnosis.
type ToolRecord struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

// ReceiptPath keeps arbitrary identity bytes out of state directory paths.
func ReceiptPath(stateDir, identity string) string {
	return filepath.Join(stateDir, "setup", install.Digest([]byte(identity))+".json")
}

// ReadReceipt preserves absence while refusing ambiguous ownership evidence.
func ReadReceipt(path string) (Receipt, error) {
	data, err := readReceiptBytes(path)
	if err != nil {
		return Receipt{}, receiptError(path, err)
	}
	receipt, err := decodeReceipt(data)
	if err != nil {
		return Receipt{}, receiptError(path, err)
	}
	return receipt, nil
}

// WriteReceipt validates before publishing so a refusal keeps the old evidence.
func WriteReceipt(path string, receipt Receipt) error {
	data, err := encodeReceipt(&receipt)
	if err != nil {
		return receiptError(path, err)
	}
	if err := receiptTarget(path); err != nil {
		return receiptError(path, err)
	}
	if err := install.WriteAtomic(path, data); err != nil {
		return receiptError(path, err)
	}
	return nil
}

func readReceiptBytes(path string) ([]byte, error) {
	if err := receiptTarget(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path) //nolint:gosec // The caller selects the state receipt, checked above.
	if err != nil {
		return nil, fmt.Errorf("open receipt: %w", err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, model.MaxPolicyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read receipt: %w", err)
	}
	if len(data) > model.MaxPolicyBytes {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrReceipt, model.MaxPolicyBytes)
	}
	return data, nil
}

func decodeReceipt(data []byte) (Receipt, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var receipt Receipt
	if err := decoder.Decode(&receipt); err != nil {
		return Receipt{}, fmt.Errorf("%w: decode JSON: %w", ErrReceipt, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return Receipt{}, fmt.Errorf("%w: trailing JSON", ErrReceipt)
	}
	if err := validateJSONKeys(json.NewDecoder(bytes.NewReader(data))); err != nil {
		return Receipt{}, err
	}
	if err := validateReceipt(&receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func encodeReceipt(receipt *Receipt) ([]byte, error) {
	if err := validateReceipt(receipt); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: encode JSON: %w", ErrReceipt, err)
	}
	if len(data) > model.MaxPolicyBytes {
		return nil, fmt.Errorf("%w: exceeds %d bytes", ErrReceipt, model.MaxPolicyBytes)
	}
	return data, nil
}
