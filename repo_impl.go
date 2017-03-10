package pod

import (
	"errors"
	"io/ioutil"
	"path"

	"github.com/go-hayden-base/fs"
)

const (
	ENUM_POD_LEVEL_REPO = iota
	ENUM_POD_LEVEL_MODULE
	ENUM_POD_LEVEL_VERSION
)

type PodLevel int

// ** Pod Impl **
func (s *Pod) Print() {
	for _, repo := range s.PodRepos {
		println("\n======== " + repo.Name + "========")
		for _, module := range repo.Modules {
			println("->" + repo.Name)
			for _, version := range module.Versions {
				println(" |_" + version.Name + " [" + path.Join(version.Root, version.FileName) + "]")
			}
		}
	}
}

func (s *Pod) index(root string, repos []string, filterFunc func(p string, level PodLevel) bool) error {
	if !fs.DirectoryExists(root) {
		return errors.New("Pod根目录不存在[" + root + "]")
	}
	if repos == nil || len(repos) == 0 {
		return errors.New("没有索引的仓库！")
	}
	podrepos := make([]*PodRepo, 0, len(repos))
	for _, rn := range repos {
		var reporoot string
		if rn == "master" {
			reporoot = path.Join(root, rn, "Specs")
		} else {
			reporoot = path.Join(root, rn)
		}
		if !fs.DirectoryExists(reporoot) {
			return errors.New("仓库不存在[" + reporoot + "]")
		}
		repo := new(PodRepo)
		repo.Name = rn
		repo.Root = reporoot
		if err := repo.index(filterFunc); err == nil {
			podrepos = append(podrepos, repo)
		}
	}

	s.PodRepos = podrepos
	return nil
}

// ** PodRepo Impl **
func (s *PodRepo) index(filterFunc func(p string, level PodLevel) bool) error {
	dirs, err := ioutil.ReadDir(s.Root)
	if err != nil {
		return err
	}
	l := len(dirs)
	modules := make([]*PodModule, 0, l)
	for _, f := range dirs {
		if !f.IsDir() {
			continue
		}
		if f.Name() == ".git" {
			continue
		}

		mp := path.Join(s.Root, f.Name())
		if filterFunc != nil && filterFunc(mp, ENUM_POD_LEVEL_MODULE) {
			continue
		}
		module := new(PodModule)
		module.Name = f.Name()
		module.Root = mp
		e := module.index(filterFunc)
		if e == nil {
			modules = append(modules, module)
		}
	}
	if len(modules) == 0 {
		return errors.New("Warn: No module in repo " + s.Name + " [" + s.Root + "]")
	}
	s.Modules = modules
	return nil
}

// ** PodModule Impl **
func (s *PodModule) index(filterFunc func(p string, level PodLevel) bool) error {
	dirs, err := ioutil.ReadDir(s.Root)
	if err != nil {
		return err
	}
	l := len(dirs)
	versions := make([]*PodModuleVersion, 0, l)
	for _, f := range dirs {
		if !f.IsDir() {
			continue
		}
		pwd := path.Join(s.Root, f.Name())
		if filterFunc != nil && filterFunc(pwd, ENUM_POD_LEVEL_VERSION) {
			continue
		}
		version := new(PodModuleVersion)
		version.Name = f.Name()
		version.Root = pwd
		e := version.index()
		if e == nil {
			versions = append(versions, version)
		}
	}
	if len(versions) == 0 {
		return errors.New("Warn: No version in module " + s.Name + " [" + s.Root + "]")
	}
	s.Versions = versions
	return nil
}

// ** PodModuleVersion Impl **
func (s *PodModuleVersion) index() error {
	dirs, err := ioutil.ReadDir(s.Root)
	if err != nil {
		return err
	}
	for _, fi := range dirs {
		if fi.IsDir() {
			continue
		}
		ext := path.Ext(fi.Name())
		if ext != ".podspec" && ext != ".json" {
			continue
		}
		s.FileName = fi.Name()
		break
	}
	if len(s.FileName) == 0 {
		return errors.New("Warn: Can not find spec file in " + s.Root)
	}

	return nil
}

// ** Func Public **
func PodIndex(podRoot string, repos []string, filterFunc func(p string, level PodLevel) bool) (*Pod, error) {
	aPod := new(Pod)
	if err := aPod.index(podRoot, repos, filterFunc); err != nil {
		return nil, err
	}
	return aPod, nil
}

func ResolvePodSpecs(aPod *Pod, threadNum int, callbackFunc func(aSpec []*Spec), logFunc func(success bool, msg string)) {
	if aPod == nil || callbackFunc == nil {
		return
	}
	if threadNum < 1 {
		threadNum = 1
	}

	cPipe := make(chan bool, threadNum)
	for i := 0; i < threadNum; i++ {
		cPipe <- true
	}

	asyncReadFunc := func(v *PodModuleVersion, c chan *Spec) {
		specPath := path.Join(v.Root, v.FileName)
		aSpec, err := ReadSpec(specPath)
		if err == nil {
			if logFunc != nil {
				logFunc(true, "解析Spec成功: "+specPath)
			}
			c <- aSpec
		} else {
			if logFunc != nil {
				logFunc(false, "解析Spec失败: "+specPath)
			}
			c <- nil
		}
		cPipe <- true
	}

	bat := 10 * threadNum
	cSpecs := make(chan *Spec, bat)

	dequeueFunc := func(count int, c chan *Spec) {
		specs := make([]*Spec, 0, count)
		for i := 0; i < count; i++ {
			aSpec := <-c
			if aSpec != nil {
				specs = append(specs, aSpec)
			}
		}
		callbackFunc(specs)
	}

	idx := 0
	for _, repo := range aPod.PodRepos {
		for _, module := range repo.Modules {
			for _, version := range module.Versions {
				<-cPipe
				go asyncReadFunc(version, cSpecs)
				idx++
				if idx == bat {
					dequeueFunc(bat, cSpecs)
					idx = 0
				}
			}
		}
	}

	for i := 0; i < threadNum; i++ {
		<-cPipe
	}

	dequeueFunc(idx, cSpecs)
}
