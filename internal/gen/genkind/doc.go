// Package genkind is the contract every generator kind implements: the
// request golaw new hands a kind, the input it renders from, the output the
// engine writes and the contract model the kinds read. It holds types,
// constants and interfaces only, so a kind package imports it from its first
// task and the engine and the kinds never depend on each other.
package genkind
