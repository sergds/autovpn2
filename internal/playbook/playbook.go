package playbook

import (
	"sync"

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
	Interface     string
	Hosts         []string          `yaml:",omitempty"`
	Custom        map[string]string `yaml:",omitempty"`
	PlaybookAddrs map[string]string `yaml:",omitempty"` // Used for undoing, auto-refresh
	Installed     bool              `yaml:",omitempty"`
	// Internal crap from now on
	busymtx    sync.Mutex
	busyreason string
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
	pb.busyreason = reason
	return pb.busymtx.TryLock()
}

func (pb *Playbook) Unlock() {
	pb.busyreason = ""
	pb.busymtx.Unlock()
}

func (pb *Playbook) GetLockReason() string {
	return pb.busyreason
}

func (pb *Playbook) SetInstallState(state bool) {
	pb.Installed = state
}

func (pb *Playbook) GetInstallState() bool {
	return pb.Installed
}
