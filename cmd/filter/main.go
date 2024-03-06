package main

import (
	"os"

	"git.schidlow.ski/gitea/imap-mirror/pkg/cifs"
	imapclient "git.schidlow.ski/gitea/imap-mirror/pkg/imap-client"
	imap_filter "git.schidlow.ski/gitea/imap-mirror/pkg/imap-filter"
	"git.schidlow.ski/gitea/imap-mirror/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Config struct {
	ClientConfig    imapclient.Config           `json:",inline" yaml:",inline"`
	CifsConfig      cifs.Config                 `json:",inline" yaml:",inline"`
	LuaFilterConfig imap_filter.LuaFilterConfig `json:",inline" yaml:",inline"`
}

func main() {
	log.ConfigLogger(logrus.InfoLevel)
	root := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			configFilePath, err := cmd.Flags().GetString("config.file")
			if err != nil {
				return err
			}

			configFileBytes, err := os.ReadFile(configFilePath)
			if err != nil {
				return err
			}

			cfg := Config{}
			err = yaml.Unmarshal(configFileBytes, &cfg)
			if err != nil {
				return err
			}

			cifsShare, err := cifs.OpenCifsShare(cifs.Config{
				CifsAddr:     cfg.CifsConfig.CifsAddr,
				CifsUsername: cfg.CifsConfig.CifsUsername,
				CifsPassword: cfg.CifsConfig.CifsPassword,
				CifsShare:    cfg.CifsConfig.CifsShare,
			})
			if err != nil {
				return err
			}
			defer cifsShare.Close()

			filterClient := imap_filter.NewFilterClient(
				imap_filter.NewLuaFilter(cfg.LuaFilterConfig, func(dir string) ([]string, error) {
					return cifsShare.ListFiles(dir)
				}, func(file string) (string, error) {
					return cifsShare.ReadFile(file)
				}),
			)

			client := imapclient.NewClient(cifsShare, cfg.ClientConfig, []imapclient.Plugin{filterClient})
			defer client.Close()
			err = client.Run(log.Log())

			filterClient.ProcessDeletions(client.GetImapClient())

			if err != nil {
				log.Log().Error(err)
			}

			return nil
		},
	}

	flags := root.PersistentFlags()
	flags.String("config.file", "config.yml", "config file path")

	root.AddCommand(&cobra.Command{
		Use: "config-structure",
		RunE: func(cmd *cobra.Command, args []string) error {
			config := Config{
				ClientConfig: imapclient.Config{
					ImapAddr:     "imap.example.com:993",
					ImapUsername: "user",
					ImapPassword: "password",
					StateDir:     "filter",
				},
				CifsConfig: cifs.Config{
					CifsAddr:     "cifs.example.com:445",
					CifsUsername: "user",
					CifsPassword: "password",
					CifsShare:    "share",
				},
				LuaFilterConfig: imap_filter.LuaFilterConfig{
					ScriptsDir: "scripts",
				},
			}

			configBytes, err := yaml.Marshal(config)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(configBytes)

			return err
		},
	})

	err := root.Execute()
	if err != nil {
		log.Log().Error(err)
	}
}
