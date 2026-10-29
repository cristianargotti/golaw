// The kit configuration lives in the tool directory, outside the package
// tree, so a bare package name resolves from the running linter, whose
// node_modules holds every pinned package.
import { realpathSync } from 'node:fs'
import { createRequire } from 'node:module'
import { pathToFileURL } from 'node:url'

const require = createRequire(realpathSync(process.argv[1]))

export async function load(name) {
  const loaded = await import(pathToFileURL(require.resolve(name)).href)
  return loaded.default ?? loaded
}

// locate answers the entry file of a package, for a configuration that
// names a package by path so the linter resolves the rest of its chain.
export function locate(name) {
  return require.resolve(name)
}
