// The strict set: the parity set plus the strict market configuration of
// the kit. The parity rules come again after every preset, so no preset
// turns a mapped rule off.
import { load } from './golaw-load.mjs'
import { HTML_FILES, JSX_FILES, TS_FILES, WEB_FILES, webFrame } from './golaw-web.mjs'

const MAX_FUNCTION_LINES = 40

const frame = await webFrame()
const [security, htmlEslint, htmlParser] = await Promise.all(
  ['eslint-plugin-security', '@html-eslint/eslint-plugin', '@html-eslint/parser'].map((name) => load(name)),
)
const scoped = (configs, files) => configs.map((config) => ({ ...config, files }))
const htmlRecommended = htmlEslint.configs['flat/recommended'] ?? htmlEslint.configs.recommended

export default [
  ...frame.base,
  ...scoped(frame.tseslint.configs.strictTypeChecked, TS_FILES),
  ...scoped(frame.tseslint.configs.stylisticTypeChecked, TS_FILES),
  { files: WEB_FILES, rules: frame.unicorn.configs.recommended.rules },
  { files: JSX_FILES, rules: frame.jsxA11y.flatConfigs.strict.rules },
  {
    files: WEB_FILES,
    plugins: { security },
    rules: {
      ...security.configs.recommended.rules,
      // It reports every obj[key], a false finding on ordinary code.
      'security/detect-object-injection': 'off',
      curly: ['error', 'all'],
      'max-lines-per-function': ['error', { max: MAX_FUNCTION_LINES, skipBlankLines: true, skipComments: true, IIFEs: true }],
    },
  },
  {
    // The markup parser and eslint-plugin-html share the file, the manual
    // configuration the eslint-plugin-html README documents.
    files: HTML_FILES,
    plugins: { '@html-eslint': htmlEslint },
    languageOptions: { parser: htmlParser },
    rules: {
      ...htmlRecommended.rules,
      '@html-eslint/require-lang': 'error',
      '@html-eslint/require-img-alt': 'error',
    },
  },
  ...frame.parity,
  ...frame.closing,
]
