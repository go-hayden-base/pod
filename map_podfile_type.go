package pod

type MapPodfile map[string]*MapPodfileModule

type MapPodfileModule struct {
	Name            string
	Version         string
	UpdateToVersion string
	NewestVersion   string
	IsCommon        bool
	IsNew           bool
	IsImplicit      bool
	IsLocal         bool
	Depends         []*DependBase

	flattenDepends []string
	hasSetDepends  bool
}
