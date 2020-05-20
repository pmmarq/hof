package cmdsecret

import (
	"fmt"
	"os"

	"strings"

	"github.com/spf13/cobra"

	"github.com/hofstadter-io/hof/cmd/hof/ga"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/format"

	"github.com/hofstadter-io/hof/lib/runtime"
)

var getLong = `print a secret or value(s) at path(s)`

func GetRun(args []string) (err error) {

	// you can safely comment this print out
	// fmt.Println("not implemented")

	if len(args) == 0 {
		val, err := runtime.GetRuntime().SecretGet("")
		if err != nil {
			return err
		}

		z := cue.Value{}
		if val == z {
			return fmt.Errorf("no config found, use 'hof config -h' to learn create and use configurations")
		}

		bytes, err := format.Node(val.Syntax())
		if err != nil {
			return err
		}
		fmt.Println(string(bytes))
		return nil
	}

	for _, a := range args {
		val, err := runtime.GetRuntime().SecretGet(a)
		if err != nil {
			return err
		}

		bytes, err := format.Node(val.Syntax())
		if err != nil {
			return err
		}
		fmt.Printf("%s: %s\n\n", a, string(bytes))
	}

	return nil
}

var GetCmd = &cobra.Command{

	Use: "get <key.path>",

	Short: "print a secret or value(s) at path(s)",

	Long: getLong,

	PreRun: func(cmd *cobra.Command, args []string) {

		cs := strings.Fields(cmd.CommandPath())
		c := strings.Join(cs[1:], "/")
		ga.SendGaEvent(c, "<omit>", 0)

	},

	Run: func(cmd *cobra.Command, args []string) {
		var err error

		// Argument Parsing

		err = GetRun(args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	},
}

func init() {

	help := GetCmd.HelpFunc()
	usage := GetCmd.UsageFunc()

	thelp := func(cmd *cobra.Command, args []string) {
		cs := strings.Fields(cmd.CommandPath())
		c := strings.Join(cs[1:], "/")
		ga.SendGaEvent(c+"/help", "<omit>", 0)
		help(cmd, args)
	}
	tusage := func(cmd *cobra.Command) error {
		cs := strings.Fields(cmd.CommandPath())
		c := strings.Join(cs[1:], "/")
		ga.SendGaEvent(c+"/usage", "<omit>", 0)
		return usage(cmd)
	}
	GetCmd.SetHelpFunc(thelp)
	GetCmd.SetUsageFunc(tusage)

}