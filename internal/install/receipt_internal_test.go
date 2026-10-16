package install

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestReadReceiptRefusesAnUnknownOrAnonymousRecord(t *testing.T) {
	t.Parallel()
	for name, receipt := range map[string]Receipt{
		"schema 2":      {Schema: 2, ProjectID: "project", Bundle: "bundle"},
		"no project id": {Schema: 1, Bundle: "bundle"},
		"no bundle":     {Schema: 1, ProjectID: "project"},
	} {
		if _, err := ReadReceipt(writeReceipt(t, receipt)); !errors.Is(err, ErrSettings) {
			t.Errorf("receipt with %s accepted: %v", name, err)
		}
	}
	got, err := ReadReceipt(writeReceipt(t, Receipt{Schema: 1, ProjectID: "project", Bundle: "bundle"}))
	if err != nil || got.ProjectID != "project" {
		t.Fatalf("valid receipt: %#v, %v", got, err)
	}
	unknown := filepath.Join(t.TempDir(), "receipt.json")
	must(t, os.WriteFile(unknown, []byte(`{"schema":1,"project_id":"project","bundle":"bundle","extra":1}`), 0o600))
	if _, err := ReadReceipt(unknown); err == nil {
		t.Fatal("a receipt with an unknown field was accepted")
	}
}

func writeReceipt(t *testing.T, receipt Receipt) string {
	t.Helper()
	data, err := json.Marshal(receipt)
	must(t, err)
	path := filepath.Join(t.TempDir(), "receipt.json")
	must(t, os.WriteFile(path, data, 0o600))
	return path
}
