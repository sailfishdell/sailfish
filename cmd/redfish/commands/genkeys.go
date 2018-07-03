// Copyright © 2018 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"net"
    "fmt"
    "os"

	"github.com/spf13/cobra"

	"github.com/superchalupa/go-redfish/src/tlscert"
	"github.com/superchalupa/go-redfish/src/log"
)

func init() {
	rootCmd.AddCommand(genkeysCmd)
}

// genkeysCmd represents the genkeys command
var genkeysCmd = &cobra.Command{
	Use:   "genkeys",
	Short: "Generate SSL keys",
	Long: `The genkeys command will generate SSL server keys for the redfish server to use.`,
	Run: func(cmd *cobra.Command, args []string) {

        fmt.Fprintf(os.Stderr, "TEST123\n")
        newLogger := logger.New("module", "happy")
        fmt.Fprintf(os.Stderr, "TEST999\n")

		ca, _ := tlscert.NewCert(
			tlscert.CreateCA,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("CA Cert common name"),
			tlscert.SetSerialNumber(12345),
			tlscert.SetBaseFilename("ca"),
			tlscert.GenRSA(2048),
			tlscert.SelfSigned(),
			tlscert.WithLogger(newLogger),
		)
		ca.Serialize()


		serverCert, _ := tlscert.NewCert(
			tlscert.GenRSA(2048),
			tlscert.SignWithCA(ca),
			tlscert.MakeServer,
			tlscert.ExpireInOneYear,
			tlscert.SetCommonName("localhost"),
			tlscert.SetSubjectKeyID([]byte{1, 2, 3, 4, 6}),
			tlscert.AddSANDNSName("localhost", "localhost.localdomain"),
			tlscert.SetSerialNumber(12346),
			tlscert.SetBaseFilename("server"),
			tlscert.WithLogger(logger),
		)
		iterInterfaceIPAddrs(logger, func(ip net.IP) { serverCert.ApplyOption(tlscert.AddSANIP(ip)) })
		serverCert.Serialize()

	},
}

func iterInterfaceIPAddrs(logger log.Logger, fn func(net.IP)) {
	ifaces, _ := net.Interfaces()
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			logger.Debug("Adding local IP Address to server cert as SAN", "ip", ip, "module", "main")
			fn(ip)
		}
	}
}
