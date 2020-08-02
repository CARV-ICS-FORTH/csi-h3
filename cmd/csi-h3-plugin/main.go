package main

import (
	"flag"
	"fmt"
	"os"
	"github.com/spf13/cobra"
	"github.com/chazapis/csi-h3/pkg/h3"
)

var (
	endpoint string
	nodeID   string
)

func init() {
	flag.Set("logtostderr", "true")
}

func main() {

	flag.CommandLine.Parse([]string{})

	cmd := &cobra.Command{
		Use:   "H3",
		Short: "CSI based H3 driver",
		Run: func(cmd *cobra.Command, args []string) {
			handle()
		},
	}

	cmd.Flags().AddGoFlagSet(flag.CommandLine)

	cmd.PersistentFlags().StringVar(&nodeID, "nodeid", "", "node id")
	cmd.MarkPersistentFlagRequired("nodeid")

	cmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "CSI endpoint")
	cmd.MarkPersistentFlagRequired("endpoint")

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Prints information about this version of csi H3 plugin",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf(`csi-h3 plugin
Version:    %s
`, h3.DriverVersion)
		},
	}

	cmd.AddCommand(versionCmd)
	versionCmd.ResetFlags()

	cmd.ParseFlags(os.Args[1:])
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s", err.Error())
		os.Exit(1)
	}

	os.Exit(0)
}

func handle() {
	d := h3.NewDriver(nodeID, endpoint)
	d.Run()
}
