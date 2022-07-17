package imap_client

type Config struct {
	ImapAddr     string `json:"imapAddr" yaml:"imapAddr"`
	ImapUsername string `json:"imapUsername" yaml:"imapUsername"`
	ImapPassword string `json:"imapPassword" yaml:"imapPassword"`
	CifsAddr     string `json:"cifsAddr" yaml:"cifsAddr"`
	CifsUsername string `json:"cifsUsername" yaml:"cifsUsername"`
	CifsPassword string `json:"cifsPassword" yaml:"cifsPassword"`
	CifsShare    string `json:"cifsShare" yaml:"cifsShare"`
	BackupDir    string `json:"backupDir" yaml:"backupDir"`
}
