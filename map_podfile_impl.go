package pod

import (
	"strings"

	"errors"

	"strconv"

	"bytes"

	fdt "github.com/go-hayden-base/foundation"
	ver "github.com/go-hayden-base/version"
)

func NewMapPodfile(aPodfile *Podfile, target string, updateRule map[string]string, qvFunc QueryVersionFunc, qdFunc QueryDependsFunc) (*MapPodfile, error) {
	if aPodfile == nil {
		return nil, errors.New("Argement aPodfile is nil")
	}
	aMapPodfile := new(MapPodfile)
	aMapPodfile.Map = make(map[string]*MapPodfileModule)
	aMapPodfile.sameParentMap = make(map[string]map[string]*MapPodfileModule)

	aMapPodfile.updateRule = updateRule
	aMapPodfile.queryVersionFunc = qvFunc
	aMapPodfile.queryDependsFunc = qdFunc

	aTarget := aPodfile.TargetWithName(target)
	if aTarget != nil {
		for _, aModule := range aTarget.Modules {
			aMapModule := NewMapPodfileModule(aModule)
			aMapPodfile.Map[aMapModule.Name] = aMapModule
		}
	}
	aMapPodfile.versionifyRuleIfNeeds()
	aMapPodfile.convertConstraintIfNeeds()
	return aMapPodfile, nil
}

func NewMapPodfileModule(aModule *PodfileModule) *MapPodfileModule {
	aMapModule := new(MapPodfileModule)
	aMapModule.Name = aModule.N
	aMapModule.OriginV = aModule.V
	if aModule.IsLocal() {
		aMapModule.AddState(StateMapPodfileModuleLocal)
		if len(aModule.Depends) > 0 {
			aMapModule.setDepends(nil)
		} else {
			aMapModule.setDepends(aModule.Depends)
		}
	}
	return aMapModule
}

// ** MapPodfile Impl **

// Evolution podfile
func (s *MapPodfile) Evolution(logFunc func(msg string)) error {
	s.evolutionTimes++
	if logFunc != nil {
		logFunc("执行依赖分析[ 第" + strconv.FormatUint(uint64(s.evolutionTimes), 10) + "次迭代 ] ...")
	}
	s.fillNewestVersion()
	s.buildSameParentMap()
	if err := s.singleModuleEvolution(); err != nil {
		return err
	}
	if err := s.clusterModuleEvolution(); err != nil {
		return err
	}
	if err := s.fillDepends(); err != nil {
		return err
	}

	if s.check() {
		s.evolutionTimes = 0
		s.reduce()
		s.genBeDepended()
		return nil
	}
	return s.Evolution(logFunc)
}

func (s *MapPodfile) buildSameParentMap() {
	// Build same parent map with submodule
	for _, aModule := range s.Map {
		if strings.Index(aModule.Name, "/") < 0 {
			continue
		}
		baseName := fdt.StrSplitFirst(aModule.Name, "/")
		mm, ok := s.sameParentMap[baseName]
		if !ok {
			mm = make(map[string]*MapPodfileModule)
			s.sameParentMap[baseName] = mm
		}
		if _, ok = mm[aModule.Name]; !ok {
			mm[aModule.Name] = aModule
		}
	}

	// Add parent module to same parent map
	for _, aModule := range s.Map {
		if strings.Index(aModule.Name, "/") > -1 {
			continue
		}
		baseName := aModule.Name
		mm, ok := s.sameParentMap[baseName]
		if !ok {
			continue
		}
		if _, ok = mm[baseName]; !ok {
			mm[baseName] = aModule
		}
	}
}

func (s *MapPodfile) singleModuleEvolution() error {
	canQueryVersion := s.queryVersionFunc != nil
	for _, aModule := range s.Map {
		if strings.Index(aModule.Name, "/") > -1 {
			continue
		}
		if _, ok := s.sameParentMap[aModule.Name]; ok {
			continue
		}
		if canQueryVersion && len(aModule.constraints) > 0 {
			needsQuery := aModule.UsefulV == TagUnknownVersion
			if !needsQuery {
				needsQuery = needsQuery || !ver.MatchVersionConstrains(aModule.constraints, aModule.UsefulV)
			}
			if needsQuery {
				v, err := s.queryVersionFunc(aModule.Name, aModule.constraints)
				if err != nil {
					return err
				}

				if v == TagEmptyVersion {
					aModule.UsefulV = TagUnknownVersion
				} else {
					aModule.UsefulV = v
				}
			}
			aModule.constraints = nil
		}
	}
	return nil
}

func (s *MapPodfile) fillNewestVersion() {
	canQueryVersion := s.queryVersionFunc != nil
	for _, aModule := range s.Map {
		if aModule.NewestV != TagEmptyVersion {
			continue
		}
		if !canQueryVersion {
			aModule.NewestV = TagUnknownVersion
			continue
		}
		v, err := s.queryVersionFunc(aModule.Name, nil)
		if err != nil {
			aModule.NewestV = TagUnknownVersion
			continue
		}
		aModule.NewestV = v
	}
}

func (s *MapPodfile) clusterModuleEvolution() error {
	canQueryVersion := s.queryVersionFunc != nil
	for parent, mm := range s.sameParentMap {
		constraints, versions := make([]string, 0, 5), make([]string, 0, 2)
		for _, aModule := range mm {
			if len(aModule.constraints) > 0 {
				constraints = append(constraints, aModule.constraints...)
			}
			if aModule.UsefulV != TagEmptyVersion && aModule.UsefulV != TagUnknownVersion {
				versions = append(versions, aModule.UsefulV)
			}
		}
		useful := TagUnknownVersion
		if len(versions) == 0 {
			if canQueryVersion {
				v, err := s.queryVersionFunc(parent, constraints)
				if err != nil {
					return err
				}
				useful = v
			}
		} else {
			vs := ver.MatchConstraintsVersions(constraints, versions)
			if len(vs) > 0 {
				v, err := ver.MaxVersion("", vs...)
				if err != nil {
					return err
				}
				useful = v
			} else if canQueryVersion {
				v, err := s.queryVersionFunc(parent, constraints)
				if err != nil {
					return err
				}
				useful = v
			}
		}
		for _, aModule := range mm {
			aModule.UsefulV = useful
			aModule.constraints = nil
		}
	}
	return nil
}

func (s *MapPodfile) fillDepends() error {
	if s.queryDependsFunc == nil {
		return nil
	}
	for _, aModule := range s.Map {
		if aModule.UsefulV == TagEmptyVersion {
			return errors.New(aModule.Name + " has an empty useful version (with origin " + aModule.OriginV + " )")
		}
		if aModule.UsefulV == TagUnknownVersion {
			continue
		}
		_, ok := aModule.Depends()
		if !ok {
			depends, err := s.queryDependsFunc(aModule.Name, aModule.UsefulV)
			if err != nil {
				aModule.setDepends(nil)
			} else {
				aModule.setDepends(depends)
			}
		}
	}
	return nil
}

func (s *MapPodfile) check() bool {
	done := true
	for _, aModule := range s.Map {
		depends, ok := aModule.Depends()
		if !ok {
			continue
		}
		for _, aDepend := range depends {
			dependName := aDepend.N
			if strings.Index(dependName, "/") > -1 && strings.HasPrefix(dependName, aModule.Name) {
				continue
			}
			if aExistModule, ok := s.Map[dependName]; ok {
				if aDepend.V == TagEmptyVersion || aExistModule.UsefulV == TagEmptyVersion || aExistModule.UsefulV == TagUnknownVersion {
					continue
				}
				if !ver.MatchVersionConstraint(aDepend.V, aExistModule.UsefulV) {
					done = false
				}
				aExistModule.addConstraint(aDepend.V)
			} else {
				done = false
				aModule := new(MapPodfileModule)
				aModule.Name = aDepend.N
				aModule.OriginV = TagUnknownVersion
				aModule.UsefulV, _ = s.searchVersionFromRule(aModule.Name)
				aModule.addConstraint(aDepend.V)
				aModule.AddState(StateMapPodfileModuleNew | StateMapPodfileModuleImplicit)
				s.Map[aModule.Name] = aModule
			}
		}
	}
	return done
}

func (s *MapPodfile) reduce() {
	for _, mm := range s.sameParentMap {
		for _, aModule := range mm {
			reduce := false
			for _, aOtherModule := range mm {
				if aModule == aOtherModule {
					continue
				}
				depends, ok := aOtherModule.Depends()
				if !ok {
					continue
				}
				for _, aDepend := range depends {
					if aDepend.N == aModule.Name {
						reduce = true
						break
					}
				}
				if reduce {
					delete(mm, aModule.Name)
					delete(s.Map, aModule.Name)
					break
				}
			}
		}
	}
}

func (s *MapPodfile) genBeDepended() {
	for _, aModule := range s.Map {
		for _, aOtherModule := range s.Map {
			if aModule == aOtherModule {
				continue
			}
			depends, ok := aOtherModule.Depends()
			if !ok {
				continue
			}
			for _, aDepend := range depends {
				if aDepend.N == aModule.Name {
					aModule.beDepended++
					break
				}
			}
		}
	}
}

// *** MapPodfile - Exec after new ***
func (s *MapPodfile) versionifyRuleIfNeeds() {
	if s.updateRule == nil {
		return
	}
	for module, version := range s.updateRule {
		s.updateRule[module] = s.versionify(module, version)
	}
}

func (s *MapPodfile) convertConstraintIfNeeds() {
	for _, aModule := range s.Map {
		aModule.OriginV = s.versionify(aModule.Name, aModule.UsefulV)
		if version, ok := s.searchVersionFromRule(aModule.Name); ok {
			aModule.UsefulV = version
			continue
		}
		aModule.UsefulV = aModule.OriginV
	}
}

// *** MapPodfile - Private Utils ***
func (s *MapPodfile) searchVersionFromRule(module string) (string, bool) {
	if s.updateRule == nil {
		return TagUnknownVersion, false
	}
	baseName := fdt.StrSplitFirst(module, "/")
	version, ok := s.updateRule[baseName]
	if ok && version != TagUnknownVersion {
		return version, true
	}
	baseRoot := baseName + "/"
	for ruleModule, version := range s.updateRule {
		if strings.HasPrefix(ruleModule, baseRoot) && version != TagUnknownVersion {
			return version, true
		}
	}
	return TagUnknownVersion, false
}

func (s *MapPodfile) versionify(module, version string) string {
	if version != TagEmptyVersion && ver.IsVersion(version) {
		return version
	} else if s.queryVersionFunc != nil {
		if v, err := s.queryVersionFunc(module, []string{version}); err == nil && v != TagEmptyVersion {
			return v
		}
	}
	return TagUnknownVersion
}

// ** MapPodfileModule Impl **
func (s *MapPodfileModule) UpgradeTag() string {
	c := ver.CompareVersion(s.OriginV, s.UsefulV)
	switch c {
	case -1:
		return "+"
	case 1:
		return "-"
	default:
		return "="
	}
}

// *** MapPodfileModule - Depends ***
func (s *MapPodfileModule) Depends() ([]*DependBase, bool) {
	if s.verDepMap == nil {
		return nil, false
	}
	res, ok := s.verDepMap[s.UsefulV]
	return res, ok
}

func (s *MapPodfileModule) setDepends(depends []*DependBase) {
	if s.verDepMap == nil {
		s.verDepMap = make(map[string][]*DependBase)
	}
	s.verDepMap[s.UsefulV] = depends
}

// *** MapPodfileModule - Constaints ***
func (s *MapPodfileModule) addConstraint(c string) {
	if c == TagEmptyVersion || c == TagUnknownVersion {
		return
	}
	if !ver.IsVersionConstraint(c) || fdt.SliceContainsStr(c, s.constraints) {
		return
	}
	if s.constraints == nil {
		s.constraints = make([]string, 0, 5)
	}
	s.constraints = append(s.constraints, c)
}

// *** MapPodfileModule - State ***
func (s *MapPodfileModule) State() uint {
	return s.state
}

func (s *MapPodfileModule) AddState(state uint) {
	if state < 0 {
		return
	}
	s.state = s.state | state
}

func (s *MapPodfileModule) RemoveState(state uint) {
	if state < 0 {
		return
	}
	s.state = s.state & ^state
}

func (s *MapPodfileModule) IsLocal() bool {
	return (s.state & StateMapPodfileModuleLocal) == StateMapPodfileModuleLocal
}

func (s *MapPodfileModule) IsCommon() bool {
	return (s.state & StateMapPodfileModuleCommon) == StateMapPodfileModuleCommon
}

func (s *MapPodfileModule) IsNew() bool {
	return (s.state & StateMapPodfileModuleNew) == StateMapPodfileModuleNew
}

func (s *MapPodfileModule) IsImplicit() bool {
	return (s.state & StateMapPodfileModuleImplicit) == StateMapPodfileModuleImplicit
}

// *** MapPodfileModule - beDepended ***
func (s *MapPodfileModule) BeDepended() int {
	return s.beDepended
}

// *** MapPodfileModule - To string ***
func (s *MapPodfileModule) DependsString() string {
	depends, ok := s.Depends()
	if !ok || len(depends) == 0 {
		return ""
	}
	var buffer bytes.Buffer
	for _, aDepend := range depends {
		buffer.WriteString("[" + aDepend.N)
		if aDepend.V != "" {
			buffer.WriteString(" " + aDepend.V)
		}
		buffer.WriteString("] ")
	}
	return buffer.String()
}
