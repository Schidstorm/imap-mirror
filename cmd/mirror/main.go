package main

import (
	imapclient "git.schidlow.ski/gitea/imap-mirror/pkg/imap-client"
	"git.schidlow.ski/gitea/imap-mirror/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
	"io/ioutil"
)

func main() {
	log.ConfigLogger(logrus.InfoLevel)
	root := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			configFilePath, err := cmd.Flags().GetString("config.file")
			if err != nil {
				return err
			}

			configFileBytes, err := ioutil.ReadFile(configFilePath)
			if err != nil {
				return err
			}

			cfg := imapclient.Config{}
			err = yaml.Unmarshal(configFileBytes, &cfg)
			if err != nil {
				return err
			}

			client := new(imapclient.Client)
			err = client.Run(log.Log(), cfg)

			if err != nil {
				log.Log().Error(err)
			}

			return nil
		},
	}

	flags := root.PersistentFlags()
	flags.String("config.file", "config.yml", "config file path")

	err := root.Execute()
	if err != nil {
		log.Log().Error(err)
	}
}
