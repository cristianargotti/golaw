// Package toolchain pins the external linters of the non-Go surface (M6.5):
// the table, the tool directory and its receipts, the resolver, the process
// runner with its budgets and the installer `golaw setup --surface` runs.
// Nothing here touches the network except Install, through the fetcher its
// caller passes.
package toolchain
