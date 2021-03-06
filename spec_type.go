package pod

type Spec struct {
	FilePath        string           `json:"-" bson:"-"`
	DefaultSpecsMap map[string]*Spec `json:"-" bson:"-"`
	SingleSpecsMap  map[string]*Spec `json:"-" bson:"-"`
	ModulePath      string           `json:"-" bson:"-"`
	hasHash         bool             `json:"-" bson:"-"`

	Name         string          `json:"name,omitempty" bson:"name,omitempty"`
	Version      string          `json:"version,omitempty" bson:"version,omitempty"`
	Platforms    *SpecPlatform   `json:"platforms,omitempty" bson:"platforms,omitempty"`
	Source       *SpecSource     `json:"source,omitempty" bson:"source,omitempty"`
	DefaultSpecs interface{}     `json:"default_subspecs,omitempty" bson:"default_subspecs,omitempty"`
	Dependences  SpecDenpendence `json:"dependencies,omitempty" bson:"dependencies,omitempty"`
	Subspecs     []*Spec         `json:"subspecs,omitempty" bson:"subspecs,omitempty"`
}

type SpecPlatform struct {
	IOS string `json:"ios,omitempty" bson:"ios,omitempty"`
}

type SpecSource struct {
	Git    string `json:"git,omitempty" bson:"git,omitempty"`
	Tag    string `json:"tag,omitempty" bson:"tag,omitempty"`
	Branch string `json:"branch,omitempty" bson:"branch,omitempty"`
	Commit string `json:"commit,omitempty" bson:"commit,omitempty"`
}

type SpecDenpendence map[string][]string
