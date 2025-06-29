package main

import (
	"fmt"
	"os"
	"time"

	"git.schidlow.ski/gitea/imap-mirror/pkg/cifs"
	imap_backup "git.schidlow.ski/gitea/imap-mirror/pkg/imap-backup"
	imapclient "git.schidlow.ski/gitea/imap-mirror/pkg/imap-client"
	imap_filter "git.schidlow.ski/gitea/imap-mirror/pkg/imap-filter"
	logger "git.schidlow.ski/gitea/imap-mirror/pkg/log"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type Config struct {
	RunPeriode *time.Duration `json:"runPeriode" yaml:"runPeriode"`

	ImapAddr                string `json:"imapAddr" yaml:"imapAddr"`
	ImapUsername            string `json:"imapUsername" yaml:"imapUsername"`
	ImapPassword            string `json:"imapPassword" yaml:"imapPassword"`
	StateDir                string `json:"stateDir" yaml:"stateDir"`
	FilterStateFile         string `json:"filterStateFile" yaml:"filterStateFile"`
	BackupStateFile         string `json:"backupStateFile" yaml:"backupStateFile"`
	FilterLastMessageOffset uint32 `json:"filterLastMessageOffset" yaml:"filterLastMessageOffset"`

	CifsConfig      cifs.Config                 `json:",inline" yaml:",inline"`
	BackupConfig    imap_backup.Config          `json:",inline" yaml:",inline"`
	LuaFilterConfig imap_filter.LuaFilterConfig `json:",inline" yaml:",inline"`
}

func main() {
	logger.Configure(log.DebugLevel)
	root := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			for {
				cfg, err := loadConfig(cmd.Flag("config.file").Value.String())
				if err != nil {
					log.WithError(err).Error("Failed to load config. Retrying in 5 seconds")
					time.Sleep(5 * time.Second)
					continue
				}

				err = daemon(cfg)
				if err != nil {
					log.Error(err)
				}

				if cfg.RunPeriode == nil || *cfg.RunPeriode == 0 {
					break
				}

				time.Sleep(*cfg.RunPeriode)
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
				ImapAddr:        "imap.example.com:993",
				ImapUsername:    "user",
				ImapPassword:    "password",
				StateDir:        "state",
				FilterStateFile: "filter_state.json",
				BackupStateFile: "backup_state.json",

				CifsConfig: cifs.Config{
					CifsAddr:     "cifs.example.com:445",
					CifsUsername: "user",
					CifsPassword: "password",
					CifsShare:    "share",
				},
				BackupConfig: imap_backup.Config{
					BackupDir: "backup",
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
		log.Error(err)
	}
}

func loadConfig(configFilePath string) (Config, error) {
	configFileBytes, err := os.ReadFile(configFilePath)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{}
	err = yaml.Unmarshal(configFileBytes, &cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func daemon(cfg Config) (resultErr error) {
	defer func() {
		if r := recover(); r != nil {
			var err error
			if rErr, ok := r.(error); ok {
				err = rErr
			} else {
				err = fmt.Errorf("%v", r)
			}

			resultErr = fmt.Errorf("cought panic: %w", err)
			return
		}
	}()

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

	return runClient(cifsShare, cfg)
}

func runClient(cifsShare cifs.CifsShare, cfg Config) error {
	log.Info("Running client")

	filterClient := imap_filter.NewFilterClient(
		imap_filter.NewLuaFilter(cfg.LuaFilterConfig, func(dir string) ([]string, error) {
			return cifsShare.ListFiles(dir)
		}, func(file string) (string, error) {
			return cifsShare.ReadFile(file)
		}),
	)

	backupClient := imap_backup.NewImapBackup(cifsShare, cfg.BackupConfig)

	client := imapclient.NewClient(cifsShare, imapclient.Config{
		ImapAddr:     cfg.ImapAddr,
		ImapUsername: cfg.ImapUsername,
		ImapPassword: cfg.ImapPassword,
		StateDir:     cfg.StateDir,
		StateFile:    &cfg.BackupStateFile,
	}, []imapclient.HandleMessagePlugin{backupClient, filterClient})
	defer client.Close()

	err := client.Open()
	if err != nil {
		return err
	}

	filterClient.SetConnection(client.GetConnection())

	return client.Run()
}
