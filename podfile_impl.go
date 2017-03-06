package pod

import (
	"os/exec"

	"path"

	fdt "github.com/go-hayden-base/foundation"
	ver "github.com/go-hayden-base/version"
	yaml "gopkg.in/yaml.v2"
)

// ** Podfile Impl **
func (s *Podfile) FillLocalModuleDepends(threadNum int, logFunc func(success bool, msg string)) {
	if threadNum < 1 {
		threadNum = 1
	}
	c := make(chan bool, threadNum)
	for i := 0; i < threadNum; i++ {
		c <- true
	}

	dir := path.Dir(s.FilePath)
	asyncFunc := func(aModule *PodfileModule) {
		specPath := path.Join(dir, aModule.SpecPath)
		aSpec, err := ReadSpec(specPath)
		if err != nil {
			if logFunc != nil {
				logFunc(false, "解析Spec失败: "+specPath+" 原因: "+err.Error())
			}
		} else {
			aModule.V = aSpec.Version
			aModule.Depends = getAllDependsFromSpec(aSpec)
		}
		c <- true
	}

	for _, aTarget := range s.Targets {
		for _, aModule := range aTarget.Modules {
			if !aModule.IsLocal() {
				continue
			}
			<-c
			go asyncFunc(aModule)
		}
	}
	for i := 0; i < threadNum; i++ {
		<-c
	}
}

func (s *Podfile) MapPodfileWithTarget(target string) MapPodfile {
	aMapPodfile := make(MapPodfile)
	if target == "" {
		for _, aTarget := range s.Targets {
			for _, aModule := range aTarget.Modules {
				aMapModule := aModule.MapPodfileModule()
				aMapPodfile[aMapModule.Name] = aMapModule
			}
		}
	} else if aTarget := s.TargetWithName(target); aTarget != nil {
		for _, aModule := range aTarget.Modules {
			aMapModule := aModule.MapPodfileModule()
			aMapPodfile[aMapModule.Name] = aMapModule
		}
	}
	return aMapPodfile
}

func (s *Podfile) HasModule(name string, inTargets []string) bool {
	for _, aTarget := range s.Targets {
		if inTargets != nil && !fdt.SliceContainsStr(aTarget.Name, inTargets) {
			continue
		}
		if aTarget.HasModule(name) {
			return true
		}
	}
	return false
}

func (s *Podfile) TargetWithName(name string) *PodfileTarget {
	for _, aTarget := range s.Targets {
		if aTarget.Name == name {
			return aTarget
		}
	}
	return nil
}

func (s *Podfile) VersionOfModule(name string, inTargets []string) (string, bool) {
	found := make([]string, 0, 5)
	exist := false
	for _, aTarget := range s.Targets {
		if inTargets != nil && !fdt.SliceContainsStr(aTarget.Name, inTargets) {
			continue
		}
		aModule := aTarget.ModuleWithFuzzyName(name)
		if aModule != nil {
			if !exist {
				exist = true
			}
			if aModule.V != "" {
				found = append(found, aModule.V)
			}
		}
	}
	if len(found) == 0 {
		return "", exist
	}
	max, err := ver.MaxVersion("", found...)
	if err != nil {
		return found[0], exist
	}
	return max, exist
}

func (s *Podfile) EnumerateAllModules(f func(target, module, version string)) {
	if f == nil {
		return
	}
	for _, aTarget := range s.Targets {
		for _, aModule := range aTarget.Modules {
			f(aTarget.Name, aModule.N, aModule.V)
		}
	}
}

func (s *Podfile) Print() {
	for _, aTarget := range s.Targets {
		println("-> " + aTarget.Name)
		for _, aModule := range aTarget.Modules {
			println("   -", aModule.Name, aModule.Version, aModule.Type, aModule.SpecPath)
		}
	}
}

// ** Target Impl **
func (s *PodfileTarget) ModuleWithName(name string) *PodfileModule {
	for _, aModule := range s.Modules {
		if aModule.N == name {
			return aModule
		}
	}
	return nil
}

func (s *PodfileTarget) ModuleWithFuzzyName(name string) *PodfileModule {
	for _, aModule := range s.Modules {
		if fdt.StrSplitFirst(aModule.N, "/") == fdt.StrSplitFirst(name, "/") {
			return aModule
		}
	}
	return nil
}

func (s *PodfileTarget) HasModule(name string) bool {
	has := false
	for _, aModule := range s.Modules {
		if aModule.N == name {
			has = true
			break
		}
	}
	return has
}

// ** PodfileModule Impl **
func (s *PodfileModule) IsLocal() bool {
	return s.SpecPath != ""
}

func (s *PodfileModule) MapPodfileModule() *MapPodfileModule {
	aModule := new(MapPodfileModule)
	aModule.Name = s.N
	aModule.Version = s.V
	aModule.Depends = s.Depends
	aModule.IsLocal = s.IsLocal()
	return aModule
}

// ** Func Public **
func NewPodfile(filePath string) (*Podfile, error) {
	b, err := exec.Command("pod", "ipc", "podfile", filePath).Output()
	if err != nil {
		return nil, err
	}
	var pf *p_podfile
	err = yaml.Unmarshal(b, &pf)
	if err != nil {
		return nil, err
	}
	aPodfile := new(Podfile)
	aPodfile.Targets = make([]*PodfileTarget, 0, 5)
	for _, a := range pf.Target_definitions {
		// 读取子Targert
		for _, b := range a.Children {
			aTarget := new(PodfileTarget)
			aTarget.Name = b.Name
			aTarget.Modules = generateModules(b.Dependencies)
			aPodfile.Targets = append(aPodfile.Targets, aTarget)
		}

		// 读取主Target
		if len(a.Dependencies) > 0 {
			aTarget := new(PodfileTarget)
			aTarget.Name = "*"
			aTarget.Modules = generateModules(a.Dependencies)
			aPodfile.Targets = append(aPodfile.Targets, aTarget)
		}
	}
	aPodfile.FilePath = filePath
	return aPodfile, nil
}

// ** Func Private **
func getAllDependsFromSpec(aSpec *Spec) []*DependBase {
	if aSpec == nil {
		return nil
	}
	mapDup := make(map[string]*DependBase)
	aSpec.enumerateDepends(func(module, depend, version string) {
		_, ok := mapDup[depend]
		if ok {
			return
		}
		aDepend := new(DependBase)
		aDepend.N = depend
		aDepend.V = version
		mapDup[depend] = aDepend
	})
	if len(mapDup) == 0 {
		return nil
	}
	res := make([]*DependBase, 0, len(mapDup))
	for _, val := range mapDup {
		res = append(res, val)
	}
	return res
}

func generateModules(module []interface{}) []*PodfileModule {
	modules := make([]*PodfileModule, 0, 5)
	for _, c := range module {
		s, ok := c.(string)
		if ok {
			aModule := new(PodfileModule)
			aModule.N = s
			modules = append(modules, aModule)
			continue
		}
		cm, ok := c.(map[interface{}]interface{})
		if !ok {
			continue
		}
		for k, v := range cm {
			ks, ok := k.(string)
			if !ok {
				continue
			}
			aModule := new(PodfileModule)
			aModule.N = ks
			modules = append(modules, aModule)

			varr, ok := v.([]interface{})
			if !ok {
				continue
			}
			if len(varr) == 0 {
				continue
			}
			d := varr[0]
			switch d.(type) {
			case string:
				aModule.V = d.(string)
			case map[interface{}]interface{}:
				x := d.(map[interface{}]interface{})
				for kk, vv := range x {
					kkk, okk := kk.(string)
					vvv, okv := vv.(string)
					if okk && okv {
						aModule.Type = kkk
						aModule.SpecPath = vvv
					}
				}
			}
		}
	}
	return modules
}
