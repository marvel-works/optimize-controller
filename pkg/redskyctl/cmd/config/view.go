package config

import (
	"fmt"

	cmdutil "github.com/redskyops/k8s-experiment/pkg/redskyctl/util"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

const (
	viewLong    = `View the Red Sky Ops configuration file`
	viewExample = ``
)

func NewViewCommand(f cmdutil.Factory, ioStreams cmdutil.IOStreams) *cobra.Command {
	o := NewConfigOptions(ioStreams)
	o.Run = o.runView

	cmd := &cobra.Command{
		Use:     "view",
		Short:   "View the configuration file",
		Long:    viewLong,
		Example: viewExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Run())
		},
	}

	return cmd
}

func (o *ConfigOptions) runView() error {
	output, err := yaml.Marshal(o.Config)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(o.Out, string(output))
	return err
}
