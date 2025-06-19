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

const operationTimeout = 30 * time.Second

type CifsShare struct {
	Connection net.Conn
	Session    *smb2.Session
	Share      *smb2.Share
}

func (c CifsShare) Close() error {
	failOnTimeout(func() {
		if c.Share != nil {
			c.Share.Umount()
		}
		if c.Session != nil {
			c.Session.Logoff()
		}
		if c.Connection != nil {
			c.Connection.Close()
		}
	})
	return nil
}

func OpenCifsShare(config Config) (CifsShare, error) {
	shareBundle := CifsShare{}
	var returnErr error

	failOnTimeout(func() {
		conn, err := net.Dial("tcp", config.CifsAddr)
		if err != nil {
			returnErr = err
			return
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
			returnErr = err
			return
		}
		logrus.Infof("dialed %s", config.CifsAddr)
		shareBundle.Session = s

		logrus.Infof("mounting %s", config.CifsShare)
		fs, err := s.Mount(config.CifsShare)
		if err != nil {
			returnErr = err
			return
		}
		shareBundle.Share = fs
		logrus.Infof("mounted %s", config.CifsShare)
	})

	return shareBundle, returnErr
}

func (c *CifsShare) ReadFile(file string) (string, error) {
	var fileContent []byte
	var err error

	failOnTimeout(func() {
		fileContent, err = c.Share.ReadFile(file)
	})

	if err != nil {
		logrus.WithError(err).Errorf("failed to read file %s", file)
	}

	return string(fileContent), nil
}

func (c *CifsShare) ListFiles(dir string) ([]string, error) {
	var result []string

	var fileInfos []fs.FileInfo
	var err error

	failOnTimeout(func() {
		fileInfos, err = c.Share.ReadDir(dir)
	})
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
func (c CifsShare) Open(name string) (f fs.File, err error) {
	failOnTimeout(func() {
		f, err = c.Share.Open(name)
	})
	return
}

func (c CifsShare) OpenFile(name string, flag int, perm os.FileMode) (fs.File, error) {
	var f fs.File
	var err error
	failOnTimeout(func() {
		f, err = c.Share.OpenFile(name, flag, perm)
	})
	return f, err
}

func (c CifsShare) MkdirAll(name string, perm os.FileMode) error {
	var err error
	failOnTimeout(func() {
		err = c.Share.MkdirAll(name, perm)
	})
	return err
}

func (c CifsShare) WriteFile(name string, data []byte, perm os.FileMode) error {
	var err error
	failOnTimeout(func() {
		err = c.Share.WriteFile(name, data, perm)
	})
	return err
}

func (c CifsShare) Chtimes(name string, atime time.Time, mtime time.Time) error {
	var err error
	failOnTimeout(func() {
		err = c.Share.Chtimes(name, atime, mtime)
	})
	return err
}

func (c CifsShare) Rename(oldpath, newpath string) error {
	// a file must be created before it can be renamed
	var content []byte
	var err error
	failOnTimeout(func() {
		content, err = c.Share.ReadFile(oldpath)
	})
	if err != nil {
		return err
	}

	failOnTimeout(func() {
		err = c.Share.WriteFile(newpath, content, os.ModePerm)
	})
	return err
}

func (c CifsShare) Remove(name string) error {
	var err error
	failOnTimeout(func() {
		err = c.Share.Remove(name)
	})
	return err
}

func (c CifsShare) Stat(name string) (fs.FileInfo, error) {
	var info fs.FileInfo
	var err error
	failOnTimeout(func() {
		info, err = c.Share.Stat(name)
	})
	return info, err
}

func failOnTimeout(f func()) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		f()
	}()

	select {
	case <-done:
		return
	case <-time.After(operationTimeout):
		panic("operation timed out")
	}
}
