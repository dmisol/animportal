package defs

type PortalConf struct {
	AnimAddr string `yaml:"anim"`
	Ram      string `yaml:"ramdisk"`

	Key    string `yaml:"key"`
	Secret string `yaml:"secret"`
	Ws     string `yaml:"ws"`

	Hall string `yaml:"hall"`

	DefaultInitJson string `yaml:"initjson"`
	DefaultFtar     string `yaml:"ftar"`

	InitialJson
}
