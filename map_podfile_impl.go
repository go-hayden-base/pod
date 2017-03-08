package pod

import (
	"bytes"
	"strconv"
	"strings"

	fdt "github.com/go-hayden-base/foundation"
	ver "github.com/go-hayden-base/version"
)

// ** GraphPodfile Impl **
func (s MapPodfile) Check() map[string][]string {
	s.banlanceVersion()
	recessive := make(map[string][]string)
	setRecessiveFunc := func(moduleName, contraint string) {
		if aDepends, ok := recessive[moduleName]; ok {
			if !fdt.SliceContainsStr(contraint, aDepends) {
				aDepends = append(aDepends, contraint)
				recessive[moduleName] = aDepends
			}
		} else {
			aDepends = make([]string, 1, 5)
			aDepends[0] = contraint
			recessive[moduleName] = aDepends
		}
	}
	for _, aModule := range s {
		depends, ok := aModule.Depends()
		if !ok {
			continue
		}
		for _, aDepend := range depends {
			dependName := aDepend.N
			if aExistModule, ok := s[dependName]; ok {
				if aExistModule.UseVersion() == "" || aExistModule.UseVersion() == "*" || aDepend.Version() == "" {
					continue
				}
				if ver.MatchVersionConstraint(aDepend.V, aExistModule.UseVersion()) {
					continue
				}
			}
			setRecessiveFunc(aDepend.N, aDepend.V)
		}
	}
	return recessive
}

func (s MapPodfile) Bytes() []byte {
	var buffer bytes.Buffer
	for _, m := range s {
		buffer.WriteString(m.Name + "," + strconv.FormatBool(m.IsCommon) + "," + strconv.FormatBool(m.IsNew) + "," + strconv.FormatBool(m.IsImplicit) + "," + strconv.FormatBool(m.IsLocal) + ",")
		buffer.WriteString(m.Version + "," + m.UpdateToVersion + "," + m.UpgradeTag() + "," + m.NewestVersion + ",")
		if depends, ok := m.Depends(); ok {
			for _, aDep := range depends {
				buffer.WriteString(aDep.String() + " ")
			}
		}
		buffer.WriteString("\n")
	}
	return buffer.Bytes()
}

func (s MapPodfile) String() string {
	return string(s.Bytes())
}

func (s MapPodfile) EnumerateAll(f func(module, current, upgradeTo, upgradeTag, newest, dependencies string, isCommon, isNew, isImplicit, isLocal bool)) {
	if f == nil {
		return
	}
	buffer := new(bytes.Buffer)
	for _, aModuel := range s {
		if depends, ok := aModuel.Depends(); ok {
			for _, aDepend := range depends {
				buffer.WriteString("[" + aDepend.N)
				if aDepend.V != "" {
					buffer.WriteString(" " + aDepend.V)
				}
				buffer.WriteString("] ")
			}
		}
		f(aModuel.Name, aModuel.Version, aModuel.UpdateToVersion, aModuel.UpgradeTag(), aModuel.NewestVersion, buffer.String(), aModuel.IsCommon, aModuel.IsNew, aModuel.IsImplicit, aModuel.IsLocal)
		buffer.Reset()
	}
}

// 平衡子模块版本号(让所有跟模块相同的模块版本保持最大版本)
func (s MapPodfile) banlanceVersion() {
	mvMap := make(map[string]string)
	// 发现父模块及其最大版本
	for moduleName, aModule := range s {
		if strings.Index(moduleName, "/") < 0 {
			continue
		}
		baseName := fdt.StrSplitFirst(moduleName, "/")
		var baseVersion string
		if baseModule, ok := s[baseName]; ok {
			baseVersion = baseModule.UseVersion()
		}
		current, ok := mvMap[baseName]
		if ok {
			if max, err := ver.MaxVersion("", baseVersion, current, aModule.UseVersion()); err == nil {
				mvMap[baseName] = max
			}
		} else {
			if max, err := ver.MaxVersion("", baseVersion, aModule.UseVersion()); err == nil {
				mvMap[baseName] = max
			} else {
				mvMap[baseName] = aModule.UseVersion()
			}
		}
		if ok {

		} else {
			mvMap[baseName] = aModule.UseVersion()
		}
	}

	// 给所有子模块赋值最大版本
	for moduleName, aModule := range s {
		if strings.Index(moduleName, "/") < 0 {
			continue
		}
		baseName := fdt.StrSplitFirst(moduleName, "/")
		if version, ok := mvMap[baseName]; ok {
			if aModule.Version != "" && aModule.Version != version {
				aModule.UpdateToVersion = version
			} else {
				aModule.Version = version
			}
		}
	}
}

// ** GraphModule Impl **
func (s *MapPodfileModule) UpgradeTag() string {
	if s.UpdateToVersion == "" || s.Version == "" {
		return "-"
	}
	c := ver.CompareVersion(s.Version, s.UpdateToVersion)
	switch c {
	case -1:
		return "+"
	case 1:
		return "-"
	default:
		return "="
	}
}

func (s *MapPodfileModule) UseVersion() string {
	if len(s.UpdateToVersion) == 0 {
		return s.Version
	} else if len(s.NewestVersion) == 0 {
		return s.UpdateToVersion
	}
	c := ver.CompareVersion(s.UpdateToVersion, s.NewestVersion)
	if c > 0 {
		return s.NewestVersion
	} else {
		return s.UpdateToVersion
	}
}

func (s *MapPodfileModule) ReferenceNodes() []string {
	if s.flattenDepends == nil {
		if depends, ok := s.Depends(); ok {
			l := len(depends)
			if l > 0 {
				s.flattenDepends = make([]string, l, l)
				for idx, aDepend := range depends {
					s.flattenDepends[idx] = aDepend.N
				}
			}
		}
	}
	return s.flattenDepends
}

func (s *MapPodfileModule) Depends() ([]*DependBase, bool) {
	if s.versionDependsMap == nil {
		return nil, false
	}
	res, ok := s.versionDependsMap[s.UseVersion()]
	return res, ok
}

func (s *MapPodfileModule) SetDepends(depends []*DependBase) {
	if s.versionDependsMap == nil {
		s.versionDependsMap = make(map[string][]*DependBase)
	}
	s.versionDependsMap[s.UseVersion()] = depends
}
