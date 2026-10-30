package main

import "testing"

func TestGreeting(t *testing.T) {
	if got := greeting("hub"); got != "hello hub" {
		t.Errorf("greeting = %q", got)
	}
}
