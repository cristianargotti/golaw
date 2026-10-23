package main

// The pt-BR lines of the decision (D20): the person who tags reads them in
// the log of the release lane and in the dry run. The causes of exit 2 are
// English, as every failure line of the kit.
const (
	lineGreen    = "o commit %s tem CI verde na main: execução %d, tentativa %d de ci.yml por push"
	lineNoRun    = "o commit %s não pode ser lançado: não há execução de ci.yml por push na main para ele; espere o CI da main ou marque um commit que o tenha"
	lineNotGreen = "o commit %s não pode ser lançado: a última execução de ci.yml por push na main (%d, tentativa %d) está %s com conclusão %s"
	noConclusion = "nenhuma"
)
