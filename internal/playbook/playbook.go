package playbook

import (
	"gopkg.in/yaml.v3"
)

type Playbook struct {
	Name     string
	Adapters struct {
		Routes string
		Dns    string
	}
	Adapterconfig struct {
		Routes map[string]string `yaml:",omitempty"`
		Dns    map[string]string `yaml:",omitempty"`
	}
	Interface          string
	Hosts              []string          `yaml:",omitempty"`
	Custom             map[string]string `yaml:",omitempty"`
	Autoupdateinterval int
	InstallTime        int64             `yaml:",omitempty"`
	PlaybookAddrs      map[string]string `yaml:",omitempty"` // Used for undoing, auto-refresh
	Installed          bool              `yaml:",omitempty"`
	Busy               bool              `yaml:",omitempty"`
	Busyreason         string            `yaml:",omitempty"`
}

func Parse(pbyaml string) (*Playbook, error) {
	pb := &Playbook{}
	err := yaml.Unmarshal([]byte(pbyaml), pb)
	if err != nil {
		return nil, err
	}
	return pb, err
}

func (pb *Playbook) Lock(reason string) bool {
	pb.Busyreason = reason
	if pb.Busy {
		return false
	}
	pb.Busy = true
	return pb.Busy
}

func (pb *Playbook) Unlock() {
	pb.Busyreason = ""
	pb.Busy = false
}

func (pb *Playbook) GetLockReason() string {
	return pb.Busyreason
}

func (pb *Playbook) SetInstallState(state bool) {
	pb.Installed = state
}

func (pb *Playbook) GetInstallState() bool {
	return pb.Installed
}
