package cifs

import (
	"io/fs"
	"net"
	"os"
	"path"
	"time"

	"github.com/hirochachacha/go-smb2"
	"github.com/sirupsen/logrus"
)

type CifsShare struct {
	Connection net.Conn
	Session    *smb2.Session
	Share      *smb2.Share
}

func (c CifsShare) Close() error {
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

	logrus.Infof("dialing %s", config.CifsAddr)
	s, err := d.Dial(conn)
	if err != nil {
		return shareBundle, err
	}
	logrus.Infof("dialed %s", config.CifsAddr)
	shareBundle.Session = s

	logrus.Infof("mounting %s", config.CifsShare)
	fs, err := s.Mount(config.CifsShare)
	if err != nil {
		return shareBundle, err
	}
	shareBundle.Share = fs
	logrus.Infof("mounted %s", config.CifsShare)

	return shareBundle, nil
}

func (c *CifsShare) ReadFile(file string) (string, error) {
	fileContent, err := c.Share.ReadFile(file)
	if err != nil {
		logrus.WithError(err).Errorf("failed to read file %s", file)
		return "", err
	}

	return string(fileContent), nil
}

func (c *CifsShare) ListFiles(dir string) ([]string, error) {
	var result []string

	fileInfos, err := c.Share.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, fileInfo := range fileInfos {
		if !fileInfo.IsDir() {
			result = append(result, path.Join(dir, fileInfo.Name()))
		} else {
			subDirFiles, err := c.ListFiles(path.Join(dir, fileInfo.Name()))
			if err != nil {
				logrus.WithError(err).Errorf("failed to list files in %s", path.Join(dir, fileInfo.Name()))
				continue
			}
			result = append(result, subDirFiles...)
		}
	}

	return result, nil
}

// implement FS interface
func (c CifsShare) Open(name string) (fs.File, error) {
	return c.Share.Open(name)
}

func (c CifsShare) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	return c.Share.OpenFile(name, flag, perm)
}

func (c CifsShare) MkdirAll(name string, perm os.FileMode) error {
	return c.Share.MkdirAll(name, perm)
}

func (c CifsShare) WriteFile(name string, data []byte, perm os.FileMode) error {
	return c.Share.WriteFile(name, data, perm)
}

func (c CifsShare) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return c.Share.Chtimes(name, atime, mtime)
}

func (c CifsShare) Rename(oldpath, newpath string) error {
	// a file must be created before it can be renamed
	content, err := c.Share.ReadFile(oldpath)
	if err != nil {
		return err
	}

	return c.Share.WriteFile(newpath, content, os.ModePerm)
}

func (c CifsShare) Remove(name string) error {
	return c.Share.Remove(name)
}

func (c CifsShare) Stat(name string) (fs.FileInfo, error) {
	return c.Share.Stat(name)
}
