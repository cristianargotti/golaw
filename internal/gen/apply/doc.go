// Package apply is the only file system writer of the engine: it lands the
// writes of one planned run atomically, file by file, inside the repository
// root and only over the bytes the plan read (M7.7, M7.9.1).
package apply
