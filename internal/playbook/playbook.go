package playbook

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
	Interface string
	Hosts     []string          `yaml:",omitempty"`
	Custom    map[string]string `yaml:",omitempty"`
}
