package bootstrap

import (
	"slices"

	"github.com/cristianargotti/golaw/internal/assets"
	"github.com/cristianargotti/golaw/internal/policy"
)

// addArrivals plans the templates whose destinations no record names, the
// files a newer kit version adds, in a repository that adopted the kit
// templates (a record names one the version renders). A kit file is planned
// as init --apply plans it: written and recorded where the tree lacks it,
// only proposed where the tree holds other bytes. A merged file the tree
// holds gets its missing lines proposed. Without this, a version whose
// strict asset names a new kit file arrives half applied: the configuration
// without the file it names. A repository that never adopted the templates
// keeps them out until init --apply, and a developer file stays the
// person's to create.
func (plan *TemplatePlan) addArrivals(files map[string]assets.Rendered, records []policy.ManagedFile) error {
	if !adopted(files, records) {
		return nil
	}
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		if slices.ContainsFunc(records, func(r policy.ManagedFile) bool { return r.Path == path }) {
			continue
		}
		arrives, err := plan.arrives(files[path])
		if err != nil {
			return err
		}
		if !arrives {
			continue
		}
		if err := plan.addTemplate(files[path]); err != nil {
			return err
		}
	}
	return plan.addRecordModules(files)
}

// adopted reports whether a record names a kit template this version
// renders, which init --apply records once it writes the templates.
func adopted(files map[string]assets.Rendered, records []policy.ManagedFile) bool {
	return slices.ContainsFunc(records, func(r policy.ManagedFile) bool {
		file, rendered := files[r.Path]
		return rendered && file.Template.Ownership == assets.OwnedByKit
	})
}

// arrives reports whether an unrecorded template joins the update: a kit
// file always, a merged file only where the tree already holds it, so an
// update never creates a file the person merges by hand.
func (plan *TemplatePlan) arrives(file assets.Rendered) (bool, error) {
	if file.Template.Ownership == assets.OwnedByKit {
		return true, nil
	}
	if file.Template.Ownership != assets.OwnedMerged {
		return false, nil
	}
	_, existed, err := readTemplate(plan.root, file.Path)
	return existed, err
}
