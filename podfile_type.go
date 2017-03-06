package pod

type Podfile struct {
	FilePath string
	Header   []byte
	Targets  []*PodfileTarget
	Footer   []byte
}

type PodfileTarget struct {
	Name    string
	Modules []*PodfileModule
}

type PodfileModule struct {
	DependBase
	Type     string
	SpecPath string
	Depends  []*DependBase
}

// *** Private ***
type p_podfile struct {
	Target_definitions []*p_target_definition
}

type p_target_definition struct {
	Abstract     bool
	Children     []*p_target
	Dependencies []interface{}
	Name         string
}

type p_target struct {
	Dependencies []interface{}
	Name         string
}
