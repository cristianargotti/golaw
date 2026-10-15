package doctor

import (
	"maps"
	"reflect"
	"slices"

	"github.com/cristianargotti/golaw/internal/install"
)

func hookLines(snapshot Snapshot) []Line {
	expected, expectedErr := install.ParseHooks(snapshot.Receipt.HookDefinition)
	actual, actualErr := install.ParseHooks([]byte(snapshot.Hooks))
	if expectedErr != nil || len(expected.Hooks) == 0 {
		return []Line{{
			ID: idHooks, State: stateRed, Code: codeHookFailOpen,
			Detail: "receipt has no valid hook definition",
		}}
	}
	lines := make([]Line, 0, len(expected.Hooks))
	for _, event := range slices.Sorted(maps.Keys(expected.Hooks)) {
		for _, group := range expected.Hooks[event] {
			line := hookLine(event, group, actual.Hooks[event])
			if actualErr != nil {
				line.State, line.Code = stateRed, codeHookFailOpen
			}
			lines = append(lines, line)
		}
	}
	return lines
}

func hookLine(event string, expected install.Group, actual []install.Group) Line {
	line := Line{
		ID: idHooks, Key: event + "/" + expected.Matcher,
		State: stateRed, Code: codeHookFailOpen,
		Detail: "missing or changed exec handler; required timeout=10 onFailure=block",
	}
	index := slices.IndexFunc(actual, func(group install.Group) bool {
		return group.Matcher == expected.Matcher
	})
	if index < 0 || !reflect.DeepEqual(actual[index], expected) {
		return line
	}
	for _, handler := range actual[index].Hooks {
		if handler.OnFailure != "block" || handler.Timeout != 10 {
			return line
		}
	}
	line.State, line.Code = stateOK, "hook-failure-blocked"
	line.Detail = "exec handler; timeout=10; onFailure=block"
	return line
}
