package toolchain

import "errors"

// The sentinel errors of the toolchain, matched with errors.Is; every one
// lives here so a caller finds the whole set in one place.
var (
	// ErrTable marks an embedded table the resolver could not trust, which
	// only a broken build of the kit can produce.
	ErrTable = errors.New("invalid toolchain table")
	// ErrNoToolsDir marks a caller that named no tool directory, so nothing
	// is written beside the repository by accident.
	ErrNoToolsDir = errors.New("no tools directory")
	// ErrConfig marks a kit configuration set that cannot be placed in the
	// tool directory.
	ErrConfig = errors.New("kit configuration tree unusable")
	// ErrReceipt marks a receipt the resolver cannot trust, so it treats
	// the tool as missing instead of running an unknown binary.
	ErrReceipt = errors.New("invalid tool receipt")
	// ErrChecksum marks a download whose bytes differ from the pin.
	ErrChecksum = errors.New("checksum mismatch")
	// ErrRuntime marks a Node or Python the npm or PyPI tools cannot run on.
	ErrRuntime = errors.New("runtime missing or below its floor")
	// ErrNoAsset marks a tool upstream publishes no binary for here.
	ErrNoAsset = errors.New("no asset for this platform")
	// ErrArchive marks an asset archive without the pinned member.
	ErrArchive = errors.New("asset archive unusable")
	// ErrInstall marks an installation step that failed after the download.
	ErrInstall = errors.New("tool installation failed")
	// ErrFetch marks a download that did not complete.
	ErrFetch = errors.New("download failed")
)
