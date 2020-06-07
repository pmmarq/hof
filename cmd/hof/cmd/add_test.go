package cmd_test

import (
	"testing"

	"github.com/hofstadter-io/hof/lib/yagu"
	"github.com/hofstadter-io/hof/script"

	"github.com/hofstadter-io/hof/cmd/hof/cmd"
)

func TestScriptAddCliTests(t *testing.T) {
	// setup some directories

	dir := "add"

	workdir := ".workdir/cli/" + dir
	yagu.Mkdir(workdir)

	script.Run(t, script.Params{
		Setup: func(env *script.Env) error {
			// add any environment variables for your tests here

			env.Vars = append(env.Vars, "HOF_TELEMETRY_DISABLED=1")

			return nil
		},
		Funcs: map[string]func(ts *script.Script, args []string) error{
			"__hof": cmd.CallTS,
		},
		Dir:         "hls/cli/add",
		WorkdirRoot: workdir,
	})
}
