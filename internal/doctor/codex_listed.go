package doctor

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
)

// sourceHooksList names the trust Codex itself answered.
const sourceHooksList = "codex app-server hooks/list"

// codexTrust asks Codex itself which installed handlers it would run and,
// when codex app-server does not answer within its deadline, reads the
// trust record instead, naming why (A20). Nothing installed starts nothing.
// With the hooks feature off Codex lists no handler at all, so the record,
// which names the feature as the cause, answers without asking Codex.
func codexTrust(ctx context.Context, input Input, lister hookLister) trustFact {
	if _, fact := readReceiptFact(input.ReceiptPath); fact.State != receiptRead {
		return trustFact{}
	}
	if config, err := readCodexConfig(filepath.Join(input.ConfigDir, codexConfigName)); err == nil && config.feature() == featureOff {
		return codexRecordTrust(input)
	}
	listed, err := lister.ListHooks(ctx, input.Environment, input.Root)
	if err == nil {
		return codexListedTrust(input, listed)
	}
	trust := codexRecordTrust(input)
	trust.Reason = err.Error()
	return trust
}

// codexListedTrust maps Codex's own verdict onto the installed handlers:
// the key of each handler at the position the hooks file holds it now,
// found in the list by the same file and the same position. A handler
// Codex does not list is skipped by Codex, which is how a project layer
// that is not trusted shows.
func codexListedTrust(input Input, listed []listedHook) trustFact {
	receipt, _ := readReceiptFact(input.ReceiptPath)
	slots, err := readSlots(receipt.HooksPath, false)
	if err != nil {
		slots = nil
	}
	trust := trustFact{Source: sourceHooksList}
	seen := false
	for _, entry := range receipt.Entries {
		current, found := handlerSlot(slots[entry.Event], entry)
		if !found {
			trust.Hooks = append(trust.Hooks, hookTrust{Key: install.CodexTrustKey(receipt.HooksPath, entry.Event, entry.Group, entry.Handler), Status: statusAbsent})
			continue
		}
		key := install.CodexTrustKey(receipt.HooksPath, entry.Event, current.Group, current.Handler)
		hook, listedNow := findListed(listed, receipt.HooksPath, install.CodexTrustKey("", entry.Event, current.Group, current.Handler))
		seen = seen || listedNow
		trust.Hooks = append(trust.Hooks, hookTrust{Key: key, Status: listedStatus(hook, listedNow)})
	}
	if input.Scope == scopeProject && !seen {
		trust.Project = projectUntrusted
	}
	return trust
}

// findListed is the listed hook of the same hooks file at the same event
// and position, the key suffix compared because Codex may spell the path
// of the file another way.
func findListed(listed []listedHook, hooksPath, suffix string) (listedHook, bool) {
	for _, hook := range listed {
		if !strings.HasSuffix(hook.Key, suffix) {
			continue
		}
		if same, err := sameFile(hook.SourcePath, hooksPath); err == nil && same { // fallback: a path that does not resolve is another file
			return hook, true
		}
	}
	return listedHook{}, false
}

// listedStatus maps the trust status of Codex: trusted and managed run,
// modified needs revalidation, and untrusted, disabled or unlisted never
// run.
func listedStatus(hook listedHook, listed bool) string {
	switch {
	case !listed || !hook.Enabled:
		return statusUntrusted
	case hook.TrustStatus == statusTrusted || hook.TrustStatus == statusManaged:
		return statusTrusted
	case hook.TrustStatus == statusModified:
		return statusModified
	default:
		return statusUntrusted
	}
}
