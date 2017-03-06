package pod

type IDepend interface {
	Version() string
	Name() string
	Subdepends() []*DependBase
	IsLocal() bool
}

type DependBase struct {
	N string
	V string
}