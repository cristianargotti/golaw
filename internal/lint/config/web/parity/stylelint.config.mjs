// The parity set of CSS: the rules of the public rule map at error, with the
// style blocks of HTML read through postcss-html.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { load } from './golaw-load.mjs'

const rules = JSON.parse(readFileSync(path.join(import.meta.dirname, 'web-rules.json'), 'utf8'))
const postcssHtml = await load('postcss-html')

export default {
  rules: rules.stylelint,
  overrides: [{ files: ['**/*.html', '**/*.htm'], customSyntax: postcssHtml }],
}
