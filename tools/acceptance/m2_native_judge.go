package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func m2JudgeNative(got result, red bool, evidence string, measured bool) error {
	if got.err != nil {
		return success(got)
	}
	output := got.stdout + "\n" + got.stderr
	if measured && m2NativeUnmeasured(output) {
		return fmt.Errorf("installed rung did not measure its checks: %w", ErrAcceptance)
	}
	if red {
		if got.code != 0 && strings.Contains(output, evidence) {
			return nil
		}
		return fmt.Errorf("installed refusal lacks %s: %w", evidence, ErrAcceptance)
	}
	if err := success(got); err != nil {
		return err
	}
	if m2NativeEnforced(output) {
		return fmt.Errorf("installed near case reported an enforced finding: %w", ErrAcceptance)
	}
	return nil
}

func m2NativeUnmeasured(output string) bool {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "unmeasured" {
			return true
		}
		if m2VerdictCount(line, "unmeasured") {
			return true
		}
		if strings.HasPrefix(line, "golaw pre-push: ") &&
			strings.Contains(line, ": govulncheck could not judge it, the pipeline judges it:") {
			return true
		}
	}
	return false
}

func m2NativeEnforced(output string) bool {
	finding := regexp.MustCompile(`(?m)^.+:\d+:\d+ (deny|fail) [a-z][a-z0-9.-]+ `)
	if finding.MatchString(output) {
		return true
	}
	for line := range strings.SplitSeq(output, "\n") {
		if m2VerdictCount(strings.TrimSpace(line), "enforced") {
			return true
		}
	}
	return false
}

func m2VerdictCount(line, status string) bool {
	if !strings.HasPrefix(line, "verdict ") {
		return false
	}
	fields := strings.Fields(line)
	for i, field := range fields {
		if i == 0 || strings.Trim(field, ",)") != status {
			continue
		}
		n, err := strconv.Atoi(strings.TrimPrefix(fields[i-1], "("))
		return err != nil || n != 0
	}
	return false
}
