package commands

import (
	"github.com/spf13/cobra"
	adapt "github.com/superchalupa/go-redfish/src/log15adapter"
)

var logCfgFile string
var logger *adapt.MyLogger

func init() {
	cobra.OnInitialize(func() {
            logger = adapt.InitializeApplicationLogging(logCfgFile)
        })
	rootCmd.PersistentFlags().StringVar(&logCfgFile, "logging-config", "", "log config file (default is /etc/redfish-logging.yaml)")
}
