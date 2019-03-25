package windows

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

type WinRunner struct {
	ServiceName        string
	ServiceDisplayName string
	ServiceHandler     svc.Handler
}

func (r *WinRunner) UnRegister() error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	// if a service can open then means it was registered
	s, err := m.OpenService(r.ServiceName)
	if err == nil {
		defer s.Close()

		err = s.Delete()
		if err != nil {
			return err
		}
	}

	// clean event log
	eventlog.Remove(r.ServiceName)

	return nil
}

func (r *WinRunner) Register() error {
	exepath, err := exePath()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()

	// if a service can open then means it was registered
	s, err := m.OpenService(r.ServiceName)
	if err == nil {
		s.Close()
		return nil
	}

	s, err = m.CreateService(
		r.ServiceName,
		exepath,
		mgr.Config{
			ServiceType:  windows.SERVICE_WIN32_OWN_PROCESS,
			StartType:    mgr.StartAutomatic,
			ErrorControl: mgr.ErrorNormal,
			DisplayName:  r.ServiceDisplayName,
		},
	)
	if err != nil {
		return err
	}
	s.Close()

	// create event log
	eventlog.InstallAsEventCreate(r.ServiceName, eventlog.Error|eventlog.Warning|eventlog.Info)

	return nil
}

func (r *WinRunner) Start() error {
	isInteractive, err := svc.IsAnInteractiveSession()
	if err != nil {
		return err
	}

	if !isInteractive {
		elog, err := eventlog.Open(r.ServiceName)
		if err != nil {
			logrus.Fatal(err)
		}

		logrus.AddHook(&ETWHook{elog})
		logrus.SetOutput(ioutil.Discard)
	}

	call := svc.Run
	if isInteractive {
		call = debug.Run
	}

	return call(r.ServiceName, r.ServiceHandler)
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			}
			return "", fmt.Errorf("%s is directory", p)
		}
	}
	return "", err
}
