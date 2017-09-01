package pod

import (
	"encoding/json"
	"errors"
)

func SpecTrimDependency(spec []byte) ([]byte, error) {
	if len(spec) == 0 {
		return nil, nil
	}
	var specObj map[string]interface{}
	if err := json.Unmarshal(spec, &specObj); err != nil {
		return nil, err
	}

	if err := funcDoTrimDependency(specObj); err != nil {
		return nil, err
	}

	newSpec, err := json.MarshalIndent(specObj, "", "    ")
	if err != nil {
		return nil, err
	}
	return newSpec, nil
}

func funcDoTrimDependency(spec map[string]interface{}) error {
	for key, val := range spec {
		if key == "dependencies" {
			delete(spec, key)
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
			if err := funcDoTrimDependency(aSubspecObj); err != nil {
				return err
			}
		}
	}
	return nil
}
