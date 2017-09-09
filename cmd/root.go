// Copyright © 2017 Juliano Martinez <juliano.martinez@martinez.io>
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

package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/ncode/pretool/shell"
	"github.com/ncode/pretool/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "pretool",
	Short: "Parallel remote execution tool",
	Long: `Parallel remote execution tool - (Yet another parallel ssh/shell)

usage:
	pretool <host1> <host2> <host3>...
`,
	Args: cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var colors = []color.Attribute{
			color.FgRed,
			color.FgGreen,
			color.FgYellow,
			color.FgBlue,
			color.FgMagenta,
			color.FgCyan,
			color.FgWhite,
			color.FgHiBlack,
			color.FgHiRed,
			color.FgHiGreen,
			color.FgHiYellow,
			color.FgHiBlue,
			color.FgHiMagenta,
			color.FgHiCyan,
			color.FgHiWhite,
		}

		for len(colors) <= len(args) {
			colors = append(colors, colors...)
		}

		hosts := []*ssh.Host{}
		for pos, hostname := range args {
			host := &ssh.Host{
				Hostname: hostname,
				Color:    color.New(colors[pos%len(colors)]),
			}
			hosts = append(hosts, host)
		}

		shell.Spawn(hosts)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pretool.yaml)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".pretool" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".pretool")
		viper.SetDefault("ssh_private_key", fmt.Sprintf("%s/.ssh/id_rsa", home))
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
