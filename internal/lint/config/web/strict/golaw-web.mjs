// The frame both web sets share: the parsers, one TypeScript program over
// the repository, the HTML script extraction and the parity rules rendered
// from the public rule map into web-rules.json. golaw lint runs ESLint with
// the repository root as its working directory.
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { load } from './golaw-load.mjs'

export const SCRIPT_FILES = ['**/*.{js,mjs,cjs,jsx,ts,mts,cts,tsx}']
const SCRIPT_EXTENSIONS = ['.js', '.mjs', '.cjs', '.jsx', '.ts', '.mts', '.cts', '.tsx']
export const JSX_FILES = ['**/*.{jsx,tsx}']
export const TS_FILES = ['**/*.{ts,mts,cts,tsx}']
export const HTML_FILES = ['**/*.{html,htm}']
export const WEB_FILES = [...SCRIPT_FILES, ...HTML_FILES]

// The React plugin cannot detect a version without React installed, and the
// rules the map names read no version-specific behavior.
const REACT_VERSION = '19.0.0'

const here = import.meta.dirname
const rules = JSON.parse(readFileSync(path.join(here, 'web-rules.json'), 'utf8'))

// The include globs of the kit program resolve from the working directory,
// so every script of the repository is in the program the typed rules read.
// The configuration parser keeps one file of each base name (panel.js hides
// panel.jsx) and ESLint then fails to parse the hidden one, so the root names
// come from the same globs read without that rule.
function program(ts) {
  const file = path.join(here, 'tsconfig.golaw.json')
  const read = ts.readConfigFile(file, ts.sys.readFile)
  const parsed = ts.parseJsonConfigFileContent(read.config, ts.sys, process.cwd())
  const scripts = ts.sys.readDirectory(process.cwd(), SCRIPT_EXTENSIONS, read.config.exclude, read.config.include)
  const rootNames = [...new Set([...parsed.fileNames, ...scripts])]
  return ts.createProgram({ rootNames, options: parsed.options })
}

export async function webFrame() {
  const names = [
    'typescript-eslint',
    'typescript',
    'eslint-plugin-sonarjs',
    'eslint-plugin-unicorn',
    'eslint-plugin-react',
    'eslint-plugin-jsx-a11y',
    'eslint-plugin-html',
  ]
  const [tseslint, ts, sonarjs, unicorn, react, jsxA11y, html] = await Promise.all(names.map((name) => load(name)))
  const base = [
    {
      files: WEB_FILES,
      plugins: { '@typescript-eslint': tseslint.plugin, unicorn, react, 'jsx-a11y': jsxA11y },
      settings: { react: { version: REACT_VERSION } },
    },
    { files: SCRIPT_FILES, languageOptions: { parser: tseslint.parser, parserOptions: { programs: [program(ts)] } } },
    { files: ['**/*.cjs', '**/*.cts'], languageOptions: { sourceType: 'commonjs' } },
    { files: HTML_FILES, plugins: { html }, languageOptions: { parser: tseslint.parser, sourceType: 'script' } },
    // The recommended configuration registers the sonarjs plugin itself, and
    // ESLint refuses a second registration of another object under its name.
    { ...sonarjs.configs.recommended, files: WEB_FILES },
  ]
  const parity = [
    { files: WEB_FILES, rules: rules.eslint.script },
    { files: JSX_FILES, rules: rules.eslint.jsx },
  ]
  // HTML scripts are outside the program, so the typed rules stay off there.
  const closing = [{ ...tseslint.configs.disableTypeChecked, files: HTML_FILES }]
  return { base, parity, closing, tseslint, unicorn, jsxA11y }
}
