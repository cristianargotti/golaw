package surfacefix

// Expectation is one finding the red twin must yield: the issue class it
// reproduces, the file, the check or lint tool, the finding code and how
// many times. The corrected twin expects none.
type Expectation struct {
	Class string
	Path  string
	Check string
	Code  string
	Count int
}

// The paths of the red twin the expectations name.
const (
	panelHTML = "views/panel.html"
	panelJSX  = "views/panel.jsx"
	hostTest  = "test/view/host.test.cjs"
	pyTest    = "test/deploy/test_install.py"
	evalPage  = "test/view/page.html"
	eslint    = "lint.eslint"
)

// The issue classes of M6.10 row (1).
const (
	classScript = "html-inline-script"
	classStyle  = "html-style"
	classJSX    = "jsx-list"
	classEval   = "cjs-eval"
	classPython = "python-test"
	classDocker = "dockerfile"
	classCFN    = "cloudformation"
	classShell  = "shell"
	preferTag   = "surface.html-prefer-tag"
	ruff        = "lint.ruff"
	template    = "infra/10-base.yml"
)

// Expectations lists the findings of the red twin under the strict set, in
// class order: the native checks by their ids and the lint tools as
// lint.<tool> with the finding code lint.Code gives their rule.
func Expectations() []Expectation {
	return []Expectation{
		{classScript, panelHTML, eslint, "sonarjs-no-unenclosed-multiline-block", 2},
		{classScript, panelHTML, eslint, "sonarjs-no-nested-conditional", 1},
		{classScript, panelHTML, eslint, "sonarjs-cognitive-complexity", 1},
		{classScript, panelHTML, eslint, "unicorn-prefer-global-this", 1},
		{classScript, panelHTML, eslint, "sonarjs-no-extra-arguments", 1},
		{classScript, panelHTML, "surface.inline-script-size", "inline-script-size", 1},
		{classScript, panelHTML, preferTag, "prefer-tag", 1},
		{classStyle, panelHTML, "lint.stylelint", "no-duplicate-selectors", 1},
		{classJSX, panelJSX, eslint, "sonarjs-unused-import", 1},
		{classJSX, panelJSX, eslint, "react-no-array-index-key", 1},
		{classJSX, panelJSX, eslint, "jsx-a11y-prefer-tag-over-role", 1},
		{classJSX, panelJSX, preferTag, "prefer-tag", 1},
		{classEval, hostTest, "surface.code-eval", "code-eval", 2},
		{classEval, hostTest, eslint, "sonarjs-code-eval", 2},
		{classEval, evalPage, "surface.code-eval", "code-eval", 1},
		{classEval, evalPage, eslint, "sonarjs-code-eval", 1},
		{classPython, pyTest, "surface.hardcoded-credential", "hardcoded-credential", 2},
		{classPython, pyTest, ruff, "s105", 1},
		{classPython, pyTest, ruff, "c420", 1},
		{classPython, pyTest, ruff, "e722", 1},
		{classDocker, "Dockerfile", "docker.tag-and-digest", "tag-and-digest", 1},
		{classDocker, "deploy/cache.Dockerfile", "docker.final-user", "final-user", 1},
		{classCFN, template, "cfn.bucket-https-only", "bucket-https-only", 1},
		{classCFN, template, "lint.checkov", "ckv-aws-18", 1},
		{classShell, "scripts/smoke.sh", "lint.shellcheck", "sc2086", 1},
	}
}
