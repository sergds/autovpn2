package playbook

type Playbook struct {
	Name      string
	Interface string
	Hosts     []string          `yaml:",omitempty"`
	Custom    map[string]string `yaml:",omitempty"`
}
