// The strict set of CSS: the standard configuration, then the parity rules
// so none is turned off. The standard configuration is extended by its path
// because the kit configuration sits outside the package tree, and stylelint
// resolves the recommended configuration it extends from its own directory:
// that package exports an import condition alone, which no require reads.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { load, locate } from './golaw-load.mjs'

const rules = JSON.parse(readFileSync(path.join(import.meta.dirname, 'web-rules.json'), 'utf8'))
const postcssHtml = await load('postcss-html')

export default {
  extends: [locate('stylelint-config-standard')],
  rules: rules.stylelint,
  overrides: [{ files: ['**/*.html', '**/*.htm'], customSyntax: postcssHtml }],
}
