package toolchain_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/toolchain"
)

func TestMissingNoticeSaysWhatWasJudgedAndTheFix(t *testing.T) {
	t.Parallel()
	ruff := mustLookup(t, "ruff")
	const path = "test/deploy/check_install.py"
	agent := "ruff 0.17.1 is not installed in the kit tool directory: test/deploy/check_install.py was judged by the native checks only; run golaw setup --surface, or the pipeline judges the rest."
	person := "ruff 0.17.1 não está instalado no diretório de ferramentas do kit: o golaw julgou test/deploy/check_install.py só com as verificações nativas; rode golaw setup --surface ou deixe o resto para o pipeline."
	tests := map[string]struct{ language, agent, person string }{
		"portuguese":                     {"pt-BR", agent, person},
		"no language is the default":     {"", agent, person},
		"another language reads english": {"en-US", agent, agent},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			gotAgent, gotPerson := toolchain.MissingNotice(ruff, path, tc.language)
			if gotAgent != tc.agent || gotPerson != tc.person {
				t.Errorf("MissingNotice = %q, %q, want %q, %q", gotAgent, gotPerson, tc.agent, tc.person)
			}
		})
	}
}
