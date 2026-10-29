// Package surface is the surface model of M6: the languages of the non-Go
// files of a repository, the file classes the native checks and the linters
// skip, the script and style regions of an HTML file, and the analyzer scope
// a remote pipeline reads, from a profile, sonar-project.properties or
// golaw.yml. It reads through model.Snapshot only and never runs a tool.
package surface
