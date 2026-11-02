package requiredtag

import (
	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func init() {
	register.Plugin("requiredtag", newPlugin)
}

// plugin adapts the analyzer to golangci-lint's module plugin system.
type plugin struct {
	settings Settings
}

func newPlugin(raw any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[Settings](raw)
	if err != nil {
		return nil, err
	}

	return plugin{settings: s}, nil
}

func (p plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	a, err := New(p.settings)
	if err != nil {
		return nil, err
	}

	return []*analysis.Analyzer{a}, nil
}

func (plugin) GetLoadMode() string { return register.LoadModeTypesInfo }
