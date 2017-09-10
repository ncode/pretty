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
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/fatih/color"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/ncode/pretool/shell"
	"github.com/ncode/pretool/ssh"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var hostsFile string
var hostGroup string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "pretool",
	Short: "Parallel remote execution tool",
	Long: `Parallel remote execution tool - (Yet another parallel ssh/shell)

usage:
	pretool <host1> <host2> <host3>...
`,
	//Args: cobra.MinimumNArgs(1),
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 && hostGroup == "" && hostsFile == "" {
			return errors.New("requires at least one host, hostGroup ou hostsFile")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		if hostGroup != "" && len(args) > 1 {
			toAppend := viper.GetStringSlice(fmt.Sprintf("groups.%s", hostGroup))
			args = append(args, toAppend...)
		} else if hostGroup != "" && len(args) < 1 {
			args = viper.GetStringSlice(fmt.Sprintf("groups.%s", hostGroup))
		}

		if hostsFile != "" {
			data, err := ioutil.ReadFile(hostsFile)
			if err != nil {
				fmt.Printf("unable to read hostsFile: %v\n", err)
				os.Exit(1)
			}
			for _, host := range strings.Split(string(data), "\n") {
				if host == "" {
					continue
				}
				args = append(args, strings.TrimSpace(host))
			}
		}

		var colors = []color.Attribute{
			color.FgRed,
			color.FgGreen,
			color.FgYellow,
			color.FgBlue,
			color.FgMagenta,
			color.FgCyan,
			color.FgWhite,
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

		hostList := ssh.NewHostList()
		for pos, hostname := range args {
			host := &ssh.Host{
				Hostname: hostname,
				Color:    color.New(colors[pos%len(colors)]),
			}
			hostList.AddHost(host)
		}
		shell.Spawn(hostList)
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
	RootCmd.PersistentFlags().StringVarP(&hostsFile, "hostsFile", "H", "", "hosts file to be used instead of the args via stdout (one host per line)")
	RootCmd.PersistentFlags().StringVarP(&hostGroup, "hostGroup", "G", "", "group of hosts to be loaded from the config file")
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
