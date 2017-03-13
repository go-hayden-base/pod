package pod

const (
	TagEmptyVersion   = ""
	TagUnknownVersion = "*"
)

const (
	StateMapPodfileModuleLocal    = uint(1)
	StateMapPodfileModuleCommon   = uint(1) << 1
	StateMapPodfileModuleNew      = uint(1) << 2
	StateMapPodfileModuleImplicit = uint(1) << 3
)

type QueryVersionFunc func(module string, constraits []string) (string, error)
type QueryDependsFunc func(module, version string) ([]*DependBase, error)

type MapPodfile struct {
	Map map[string]*MapPodfileModule

	sameParentMap    map[string]map[string]*MapPodfileModule
	updateRule       map[string]string
	queryVersionFunc QueryVersionFunc
	queryDependsFunc QueryDependsFunc
	evolutionTimes   uint
}

type MapPodfileModule struct {
	Name    string
	OriginV string
	UsefulV string
	NewestV string

	beDepended  int
	state       uint
	constraints []string // version queue
	verDepMap   map[string][]*DependBase
}
