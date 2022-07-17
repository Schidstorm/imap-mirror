package imap_client

import (
	"github.com/hirochachacha/go-smb2"
	"net"
)

type CifsShare struct {
	Connection net.Conn
	Session    *smb2.Session
	Share      *smb2.Share
}

func (c *CifsShare) Close() error {
	if c.Share != nil {
		c.Share.Umount()
	}
	if c.Session != nil {
		c.Session.Logoff()
	}
	if c.Connection != nil {
		c.Connection.Close()
	}
	return nil
}

func OpenCifsShare(config Config) (CifsShare, error) {
	shareBundle := CifsShare{}
	conn, err := net.Dial("tcp", config.CifsAddr)
	if err != nil {
		return shareBundle, err
	}
	shareBundle.Connection = conn

	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     config.CifsUsername,
			Password: config.CifsPassword,
		},
	}

	s, err := d.Dial(conn)
	if err != nil {
		return shareBundle, err
	}
	shareBundle.Session = s

	fs, err := s.Mount(config.CifsShare)
	if err != nil {
		return shareBundle, err
	}
	shareBundle.Share = fs

	return shareBundle, nil
}
