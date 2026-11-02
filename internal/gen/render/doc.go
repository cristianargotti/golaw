// Package render turns one template of a kind into formatted Go: the golaw
// header on the first line, the import block in the gci sections of the
// lint asset (standard, default, then the module) and go/format output. It
// touches no file system beyond the template tree it is handed, so a run
// is planned whole before any byte is written.
package render
