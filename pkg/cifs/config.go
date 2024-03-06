package cifs

type Config struct {
	CifsAddr     string `json:"cifsAddr" yaml:"cifsAddr"`
	CifsUsername string `json:"cifsUsername" yaml:"cifsUsername"`
	CifsPassword string `json:"cifsPassword" yaml:"cifsPassword"`
	CifsShare    string `json:"cifsShare" yaml:"cifsShare"`
}
