package pod

import (
	"encoding/json"
	"errors"
	"strings"
)

func SpecTrimDependency(spec []byte) ([]byte, error) {
	if len(spec) == 0 {
		return nil, nil
	}
	var specObj map[string]interface{}
	if err := json.Unmarshal(spec, &specObj); err != nil {
		return nil, err
	}
	rn, ok := specObj["name"]
	if !ok {
		return nil, errors.New("该Podspec没有设置名称!")
	}
	rootName, ok := rn.(string)
	if !ok {
		return nil, errors.New("转换Pod名称失败！")
	}
	if err := funcDoTrimDependency(specObj, rootName+"/"); err != nil {
		return nil, err
	}

	newSpec, err := json.MarshalIndent(specObj, "", "    ")
	if err != nil {
		return nil, err
	}
	return newSpec, nil
}

func funcDoTrimDependency(spec map[string]interface{}, rootPath string) error {
	for key, val := range spec {
		if key == "dependencies" {
			dep, ok := val.(map[string]interface{})
			if !ok {
				return errors.New("无法将依赖转换为map!")
			}
			for keyDep := range dep {
				if !strings.HasPrefix(keyDep, rootPath) {
					delete(dep, keyDep)
				}
			}
			if len(dep) == 0 {
				delete(spec, "dependencies")
			}
			continue
		}
		if key != "subspecs" {
			continue
		}
		subspecs, ok := val.([]interface{})
		if !ok {
			return errors.New("无法解析Subspecs！")
		}
		for _, aSubspec := range subspecs {
			aSubspecObj, ok := aSubspec.(map[string]interface{})
			if !ok {
				return errors.New("无法解析Subspecs的元素！")
			}
			if err := funcDoTrimDependency(aSubspecObj, rootPath); err != nil {
				return err
			}
		}
	}
	return nil
}
