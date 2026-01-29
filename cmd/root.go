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
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/ncode/pretty/internal/shell"
	"github.com/ncode/pretty/internal/sshConn"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var hostsFile string
var hostGroup string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "pretty",
	Short: "Parallel remote execution tty",
	Long: `Parallel remote execution tty - (Yet another parallel ssh/shell)

usage:
	pretty <host1> <host2> <host3>...
`,
	//Args: cobra.MinimumNArgs(1),
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 1 && hostGroup == "" && hostsFile == "" {
			return errors.New("requires at least one host, hostGroup ou hostsFile")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		argsLen := len(args)
		hostSpecs, err := parseArgsHosts(args)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		if hostGroup != "" {
			groupSpecs, err := parseGroupSpecs(viper.Get(fmt.Sprintf("groups.%s", hostGroup)), hostGroup)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			if argsLen > 1 {
				hostSpecs = append(hostSpecs, groupSpecs...)
			} else if argsLen < 1 {
				hostSpecs = groupSpecs
			}
		}

		if hostsFile != "" {
			data, err := ioutil.ReadFile(hostsFile)
			if err != nil {
				fmt.Printf("unable to read hostsFile: %v\n", err)
				os.Exit(1)
			}
			fileSpecs, err := parseHostsFile(data)
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			hostSpecs = append(hostSpecs, fileSpecs...)
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

		for len(colors) <= len(hostSpecs) {
			colors = append(colors, colors...)
		}

		userConfigPath := ""
		if home, err := os.UserHomeDir(); err == nil {
			userConfigPath = filepath.Join(home, ".ssh", "config")
		}
		resolver, err := sshConn.LoadSSHConfig(sshConn.SSHConfigPaths{
			User:   userConfigPath,
			System: "/etc/ssh/ssh_config",
		})
		if err != nil {
			fmt.Printf("unable to load ssh config: %v\n", err)
			os.Exit(1)
		}

		globalUser := strings.TrimSpace(viper.GetString("username"))

		hostList := sshConn.NewHostList()
		for pos, spec := range hostSpecs {
			resolveSpec := sshConn.HostSpec{
				Alias:   spec.Host,
				Host:    spec.Host,
				Port:    spec.Port,
				User:    spec.User,
				PortSet: spec.PortSet,
				UserSet: spec.UserSet,
			}
			if !resolveSpec.UserSet && globalUser != "" {
				resolveSpec.User = globalUser
				resolveSpec.UserSet = true
			}
			resolved, err := resolver.ResolveHost(resolveSpec, "")
			if err != nil {
				fmt.Printf("unable to resolve host %q: %v\n", spec.Host, err)
				os.Exit(1)
			}
			jumps := make([]sshConn.ResolvedHost, 0, len(resolved.ProxyJump))
			for _, jumpAlias := range resolved.ProxyJump {
				jumpSpec := sshConn.HostSpec{Alias: jumpAlias, Host: jumpAlias}
				if globalUser != "" {
					jumpSpec.User = globalUser
					jumpSpec.UserSet = true
				}
				jumpResolved, err := resolver.ResolveHost(jumpSpec, "")
				if err != nil {
					fmt.Printf("unable to resolve jump host %q: %v\n", jumpAlias, err)
					os.Exit(1)
				}
				jumps = append(jumps, jumpResolved)
			}
			displayName := hostDisplayName(HostSpec{Host: resolved.Host, Port: resolved.Port})
			host := &sshConn.Host{
				Hostname:      displayName,
				Alias:         resolved.Alias,
				Host:          resolved.Host,
				Port:          resolved.Port,
				User:          resolved.User,
				IdentityFiles: resolved.IdentityFiles,
				ProxyJump:     jumps,
				Color:         color.New(colors[pos%len(colors)]),
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
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pretty.yaml)")
	RootCmd.PersistentFlags().StringVarP(&hostsFile, "hostsFile", "H", "", "hosts file to be used instead of the args via stdout (one host per line, format: host or host:port)")
	RootCmd.PersistentFlags().StringVarP(&hostGroup, "hostGroup", "G", "", "group of hosts to be loaded from the config file")
	RootCmd.PersistentFlags().String("prompt", "", "prompt to display in the interactive shell")
	_ = viper.BindPFlag("prompt", RootCmd.PersistentFlags().Lookup("prompt"))
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

		// Search config in home directory with name ".pretty" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".pretty")
		viper.SetDefault("history_file", fmt.Sprintf("%s/.pretty.history", home))
		viper.SetDefault("ssh_private_key", fmt.Sprintf("%s/.ssh/id_rsa", home))
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
