/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.,
 * Copyright (C) 2017,-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the ",License",); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an ",AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */
package metadata

import (
	"encoding/json"
	"fmt"

	"configcenter/src/common"

	"go.mongodb.org/mongo-driver/bson"
)

/*
version: 1.0 test
description:  由于在不同的运行版本中,进程的绑定信息的二维结构中，数据的列是不一样的。 所以又下面的实现

通过定义数据反序列化的方法来实现struct 的同一个属性在不同运行版本的环境上，实现进程绑定信息的多态。
主要是利用interface 的特性来实现，

defaultPropertyBindInfoHandle，defaultProcBindInfoHandle 是在使用反序列化 进程，进程模板中进程
绑定信息实际结构的对象

下面是defaultPropertyBindInfoHandle，defaultProcBindInfoHandle中UJSON和UBSON 含义的介绍
UJSON json反序列的方法，用于HTTP的消息处理,将数据解析到不同的struct上。 这个结构需要是ProcPropertyBindInfo，ProcBindInfoInterface interface 的实现
UBSON  bson 反序列化的方法， 用于数据库存储,将数据解析到不同的struct上。这个结构需要是ProcPropertyBindInfo，ProcBindInfoInterface interface 的实现

*/

var (
	// 标准字段，不论在什么环境上都需要使用的
	ignoreField = map[string]struct{}{"template_row_id": struct{}{}, "row_id": struct{}{}, common.BKIP: struct{}{}, common.BKPort: struct{}{}, common.BKProtocol: struct{}{}, common.BKEnable: struct{}{}}
)
var (
	//  内部变量，不允许改变，改变值请用对应的Register 方案
	defaultPropertyBindInfoHandle ProcPropertyExtraBindInfoInterface = &openVersionPropertyBindInfo{}
	//  内部变量，不允许改变，改变值请用对应的Register 方案
	defaultProcBindInfoHandle ProcExtraBindInfoInterface = &openVersionProcBindInfo{}
)

// Register 实现， 替换已有的进程，进程模板中进程绑定信息实际结构的处理对象
func Register(propertyBindInfo ProcPropertyExtraBindInfoInterface, procBindInfo ProcExtraBindInfoInterface) {
	defaultPropertyBindInfoHandle = propertyBindInfo
	defaultProcBindInfoHandle = procBindInfo
}

// ProcPropertyExtraBindInfoInterface 用来处理进程模板中bind info 数据的反序列化，
// 序列号使用默认的方法，目前只支持json, bson, 如果需要其他请新加
type ProcPropertyExtraBindInfoInterface interface {
	UJSON(data []byte, bindInfo *ProcPropertyBindInfoValue) error
	UBSON(data []byte, bindInfo *ProcPropertyBindInfoValue) error
}

// ProcExtraBindInfoInterface 用来处理进程中bind info 数据的序反序列化，
// 序列号使用默认的方法，目前只支持json, bson, 如果需要其他请新加
type ProcExtraBindInfoInterface interface {
	UJSON(data []byte, bindInfo *ProcBindInfo) error
	UBSON(data []byte, bindInfo *ProcBindInfo) error
}

// ProcPropertyBindInfo 给服务模板使用的，来存储，校验服务模板中进程绑定的信息
type ProcPropertyBindInfo struct {
	// 通过Unmarshal 方法实现不同的数据类型
	Value []ProcPropertyBindInfoValue `field:"value" json:"value" bson:"value"`
	// 给前端做兼容
	AsDefaultValue *bool `field:"as_default_value" json:"as_default_value" bson:"as_default_value"`
}

// ProcPropertyBindInfoValue 给服务模板使用的，来存储，校验服务模板中进程绑定的信息, 用来做管理的
type ProcPropertyBindInfoValue struct {
	// 标准属性
	Std *stdProcPropertyBindInfoValue

	// 通过Unmarshal 方法实现不同版本中数据不一样
	extra propertyBindInfoValueInterface
}

// stdProcPropertyBindInfoValue 这个是标准的进程模板的绑定信息
type stdProcPropertyBindInfoValue struct {
	RowID    int64            `field:"row_id" json:"row_id" bson:"row_id"`
	IP       PropertyBindIP   `field:"ip" json:"ip" bson:"ip"`
	Port     PropertyPort     `field:"port" json:"port" bson:"port"`
	Protocol PropertyProtocol `field:"protocol" json:"protocol" bson:"protocol"`
	Enable   PropertyBool     `field:"enable" json:"enable" bson:"enable"`
}

type propertyBindInfoValueInterface interface {
	Validate() (string, error)
	ExtractChangeInfoBindInfo(i *ProcBindInfo) (map[string]interface{}, bool, bool)

	// ExtractInstanceUpdateData extra 主机进程bind_info中某一行的extra
	ExtractInstanceUpdateData(extra map[string]interface{}) map[string]interface{}

	// toMap  获取要保持格式的数据
	toMap() map[string]interface{}
}

// ProcBindInfo 给服务模板使用的，来存储，校验服务实例中进程绑定的信息
type ProcBindInfo struct {
	// 标准属性
	Std *stdProcBindInfo

	// extra 通过Unmarshal 方法实现不同版本中数据不一样
	extra map[string]interface{}
}

// stdProcBindInfo 这个是标准的进程实例的绑定信息
type stdProcBindInfo struct {
	TemplateRowID int64   `field:"template_row_id" json:"template_row_id" bson:"template_row_id"`
	IP            *string `field:"ip" json:"ip" bson:"ip"`
	Port          *string `field:"port" json:"port" bson:"port"`
	Protocol      *string `field:"protocol" json:"protocol" bson:"protocol"`
	Enable        *bool   `field:"enable" json:"enable" bson:"enable"`
}

/*** ProcPropertyBindInfo 依赖的方法  ****/

func (pbi *ProcPropertyBindInfo) Validate() (string, error) {
	maxRowID := int64(0)
	for idx, property := range pbi.Value {
		if property.Std == nil {
			return common.BKProcBindInfo, fmt.Errorf("not set value")
		}

		if property.Std.RowID > maxRowID {
			maxRowID = property.Std.RowID
		}

		if err := property.Std.IP.Validate(); err != nil {
			return fmt.Sprintf("%s[%d].%s", common.BKProcBindInfo, idx, common.BKIP), err
		}
		if err := property.Std.Port.Validate(); err != nil {
			return fmt.Sprintf("%s[%d].%s", common.BKProcBindInfo, idx, common.BKPort), err
		}
		if err := property.Std.Protocol.Validate(); err != nil {
			return fmt.Sprintf("%s[%d].%s", common.BKProcBindInfo, idx, common.BKProtocol), err
		}
		if err := property.Std.Enable.Validate(); err != nil {
			return fmt.Sprintf("%s[%d].%s", common.BKProcBindInfo, idx, common.BKEnable), err
		}
		if property.extra != nil {
			if fieldName, err := property.extra.Validate(); err != nil {
				return fmt.Sprintf("%s[%d].%s", common.BKProcBindInfo, idx, fieldName), err
			}
		}

	}
	for idx, property := range pbi.Value {
		if property.Std.RowID == 0 {
			maxRowID += 1
			pbi.Value[idx].Std.RowID = maxRowID
		}
	}
	return "", nil
}

func (pbi *ProcPropertyBindInfo) ExtractChangeInfoBindInfo(i *Process) ([]ProcBindInfo, bool, bool) {
	var changed, isNamePortChanged bool

	procBindInfoMap := make(map[int64]ProcBindInfo, len(i.BindInfo))
	for _, item := range i.BindInfo {
		procBindInfoMap[item.Std.TemplateRowID] = item
	}

	procBindInfoArr := make([]ProcBindInfo, 0)
	for _, row := range pbi.Value {
		inputProcBindInfo := procBindInfoMap[row.Std.RowID]

		if inputProcBindInfo.Std == nil {
			inputProcBindInfo.Std = &stdProcBindInfo{}
		}
		inputProcBindInfo.Std.TemplateRowID = row.Std.RowID

		if IsAsDefaultValue(row.Std.IP.AsDefaultValue) {
			if row.Std.IP.Value == nil && inputProcBindInfo.Std.IP != nil {
				inputProcBindInfo.Std.IP = nil
				changed = true
			} else if row.Std.IP.Value != nil && inputProcBindInfo.Std.IP == nil {
				ip := row.Std.IP.Value.IP()
				inputProcBindInfo.Std.IP = &ip
				changed = true
			} else if row.Std.IP.Value != nil && inputProcBindInfo.Std.IP != nil && row.Std.IP.Value.IP() != *inputProcBindInfo.Std.IP {
				ip := row.Std.IP.Value.IP()
				inputProcBindInfo.Std.IP = &ip
				changed = true
			}
		}

		if IsAsDefaultValue(row.Std.Port.AsDefaultValue) {
			if row.Std.Port.Value == nil && inputProcBindInfo.Std.Port != nil {
				inputProcBindInfo.Std.Port = nil
				changed = true
				isNamePortChanged = true
			} else if row.Std.Port.Value != nil && inputProcBindInfo.Std.Port == nil {
				inputProcBindInfo.Std.Port = row.Std.Port.Value
				changed = true
				isNamePortChanged = true
			} else if row.Std.Port.Value != nil && inputProcBindInfo.Std.Port != nil && *row.Std.Port.Value != *inputProcBindInfo.Std.Port {
				inputProcBindInfo.Std.Port = row.Std.Port.Value
				changed = true
				isNamePortChanged = true
			}
		}

		if IsAsDefaultValue(row.Std.Protocol.AsDefaultValue) {
			if row.Std.Protocol.Value == nil && inputProcBindInfo.Std.Protocol != nil {
				inputProcBindInfo.Std.Protocol = nil
				changed = true
			} else if row.Std.Protocol.Value != nil && inputProcBindInfo.Std.Protocol == nil {
				protocol := string(*row.Std.Protocol.Value)
				inputProcBindInfo.Std.Protocol = &protocol
				changed = true
			} else if row.Std.Protocol.Value != nil && inputProcBindInfo.Std.Protocol != nil && string(*row.Std.Protocol.Value) != *inputProcBindInfo.Std.Protocol {
				protocol := string(*row.Std.Protocol.Value)
				inputProcBindInfo.Std.Protocol = &protocol
				changed = true
			}
		}

		if IsAsDefaultValue(row.Std.Enable.AsDefaultValue) {
			if row.Std.Enable.Value == nil && inputProcBindInfo.Std.Enable != nil {
				inputProcBindInfo.Std.Enable = nil
				changed = true
			} else if row.Std.Enable.Value != nil && inputProcBindInfo.Std.Enable == nil {
				inputProcBindInfo.Std.Enable = row.Std.Enable.Value
				changed = true
			} else if row.Std.Enable.Value != nil && inputProcBindInfo.Std.Enable != nil && *row.Std.Enable.Value != *inputProcBindInfo.Std.Enable {
				inputProcBindInfo.Std.Enable = row.Std.Enable.Value
				changed = true
			}
		}

		if row.extra != nil {
			extraMap, extraChanged, isExtraNamePortChanged := row.extra.ExtractChangeInfoBindInfo(&inputProcBindInfo)
			if extraChanged {
				changed = extraChanged
			}
			if isExtraNamePortChanged {
				isNamePortChanged = isExtraNamePortChanged
			}
			inputProcBindInfo.extra = extraMap
		}
		procBindInfoArr = append(procBindInfoArr, inputProcBindInfo)
	}

	return procBindInfoArr, changed, isNamePortChanged

}

func (pbi *ProcPropertyBindInfo) ExtractInstanceUpdateData(input *Process) []ProcBindInfo {
	return pbi.changeInstanceBindInfo(input.BindInfo)
}

// changeInstanceBindInfo 根据模板和进程中的绑定信息来组成真正的进程绑定信息
func (pbi *ProcPropertyBindInfo) changeInstanceBindInfo(bindInfoArr []ProcBindInfo) []ProcBindInfo {
	procBindInfoMap := make(map[int64]ProcBindInfo, 0)

	for _, item := range bindInfoArr {
		procBindInfoMap[item.Std.TemplateRowID] = item
	}

	procBindInfoArr := make([]ProcBindInfo, 0)
	for _, row := range pbi.Value {
		inputProcBindInfo := procBindInfoMap[row.Std.RowID]
		if inputProcBindInfo.Std == nil {
			inputProcBindInfo.Std = &stdProcBindInfo{}
		}

		if row.Std == nil {
			row.Std = &stdProcPropertyBindInfoValue{}
		}

		inputProcBindInfo.Std.TemplateRowID = row.Std.RowID

		/*** 处理标准字段 ***/

		if IsAsDefaultValue(row.Std.IP.AsDefaultValue) == true {
			if row.Std.IP.Value == nil {
				inputProcBindInfo.Std.IP = nil
			} else {
				ip := row.Std.IP.Value.IP()
				inputProcBindInfo.Std.IP = &ip
			}
		}

		if IsAsDefaultValue(row.Std.Port.AsDefaultValue) == true {
			inputProcBindInfo.Std.Port = row.Std.Port.Value
		}

		if IsAsDefaultValue(row.Std.Protocol.AsDefaultValue) == true {
			if row.Std.Protocol.Value == nil {
				inputProcBindInfo.Std.Protocol = nil
			} else {
				protocol := string(*row.Std.Protocol.Value)
				inputProcBindInfo.Std.Protocol = &protocol
			}
		}

		if IsAsDefaultValue(row.Std.Enable.AsDefaultValue) == true {
			if row.Std.Enable.Value == nil {
				inputProcBindInfo.Std.Enable = nil
			} else {
				inputProcBindInfo.Std.Enable = row.Std.Enable.Value
			}
		}

		if row.extra != nil {
			inputProcBindInfo.extra = row.extra.ExtractInstanceUpdateData(inputProcBindInfo.extra)
		}

		procBindInfoArr = append(procBindInfoArr, inputProcBindInfo)
	}

	return procBindInfoArr
}

// Update  bind info 每次更新采用的是全量更新
func (pbi *ProcPropertyBindInfo) Update(input ProcessProperty, rawProperty map[string]interface{}) {
	if _, ok := rawProperty[common.BKProcBindInfo]; ok {
		pbi.AsDefaultValue = input.BindInfo.AsDefaultValue
		pbi.Value = input.BindInfo.Value
	}
	return
}

func cloneProcBindInfoArr(procBindInfoArr []ProcBindInfo) (newData []ProcBindInfo) {
	newData = make([]ProcBindInfo, len(procBindInfoArr))
	for idx, bindInfo := range procBindInfoArr {
		extra := make(map[string]interface{}, 0)
		for key, val := range bindInfo.extra {
			extra[key] = val
		}

		newData[idx] = ProcBindInfo{
			Std: &stdProcBindInfo{
				IP:       bindInfo.Std.IP,
				Port:     bindInfo.Std.Port,
				Protocol: bindInfo.Std.Protocol,
				Enable:   bindInfo.Std.Enable,
			},
			extra: extra,
		}
	}

	return
}

// Compare 对比模板和实例数据，发现数据是否变化
func (pbi *ProcPropertyBindInfo) DiffWithProcessTemplate(procBindInfoArr []ProcBindInfo) (newBindInfoArr []ProcBindInfo, change bool) {
	tmpBindInfoArr := cloneProcBindInfoArr(procBindInfoArr)
	newBindInfoArr = pbi.changeInstanceBindInfo(tmpBindInfoArr)

	if len(procBindInfoArr) != len(newBindInfoArr) {
		change = true
		return
	}

	newBindInfoKv := make(map[int64]ProcBindInfo, len(newBindInfoArr))
	for _, row := range newBindInfoArr {
		newBindInfoKv[row.Std.TemplateRowID] = row
	}

	for _, row := range procBindInfoArr {
		tmpBindInfo, ok := newBindInfoKv[row.Std.TemplateRowID]
		if !ok {
			change = true
			return
		}
		if row.Std == nil && tmpBindInfo.Std != nil ||
			row.Std != nil && tmpBindInfo.Std == nil {
			change = true
			return
		}
		if (row.Std.IP == nil && tmpBindInfo.Std.IP != nil) ||
			(row.Std.IP != nil && tmpBindInfo.Std.IP == nil) ||
			(row.Std.IP != nil && tmpBindInfo.Std.IP != nil && *row.Std.IP != *tmpBindInfo.Std.IP) {
			change = true
			return
		}
		if (row.Std.Port == nil && tmpBindInfo.Std.Port != nil) ||
			(row.Std.Port != nil && tmpBindInfo.Std.Port == nil) ||
			(row.Std.Port != nil && tmpBindInfo.Std.Port != nil && *row.Std.Port != *tmpBindInfo.Std.Port) {
			change = true
			return
		}
		if (row.Std.Protocol == nil && tmpBindInfo.Std.Protocol != nil) ||
			(row.Std.Protocol != nil && tmpBindInfo.Std.Protocol == nil) ||
			(row.Std.Protocol != nil && tmpBindInfo.Std.Protocol != nil && *row.Std.Protocol != *tmpBindInfo.Std.Protocol) {
			change = true
			return
		}
		if (row.Std.Enable == nil && tmpBindInfo.Std.Enable != nil) ||
			(row.Std.Enable != nil && tmpBindInfo.Std.Enable == nil) ||
			(row.Std.Enable != nil && tmpBindInfo.Std.Enable != nil && *row.Std.Enable != *tmpBindInfo.Std.Enable) {
			change = true
			return
		}

		if row.extra == nil && tmpBindInfo.extra != nil ||
			row.extra != nil && tmpBindInfo.extra == nil {
			change = true
			return
		}

		if len(row.extra) != len(tmpBindInfo.extra) {
			if len(row.extra) == 0 && allFieldValIsNil(tmpBindInfo.extra) {
				return
			}

			change = true
			return
		}

		for key, val := range row.extra {
			tmpVal, exist := tmpBindInfo.extra[key]
			if !exist {
				if val == nil {
					continue
				}
				change = true
				return
			}
			if val == nil && tmpVal != nil ||
				val != nil && tmpVal == nil ||
				(val != nil && tmpVal != nil && val != tmpVal) {
				change = true
				return
			}
		}

	}

	return
}

// allFieldValIsNil 判断所有的字段是否为nil
func allFieldValIsNil(extra map[string]interface{}) bool {
	isValAllNil := true
	for _, val := range extra {
		if val != nil {
			isValAllNil = false
			break
		}
	}
	return isValAllNil
}

/*** ProcPropertyBindInfoValue 依赖的方法  ****/

func (pbi *ProcPropertyBindInfoValue) UnmarshalJSON(data []byte) error {
	err := defaultPropertyBindInfoHandle.UJSON(data, pbi)
	if err != nil {
		return err
	}
	return nil
}

func (pbi *ProcPropertyBindInfoValue) UnmarshalBSON(data []byte) error {
	err := defaultPropertyBindInfoHandle.UBSON(data, pbi)
	if err != nil {
		return err
	}
	return nil
}

func (pbi ProcPropertyBindInfoValue) MarshalJSON() ([]byte, error) {
	stdData := pbi.Std.toKV()
	if pbi.extra != nil {
		stdData = merge(stdData, pbi.extra.toMap())
	}
	return json.Marshal(stdData)
}

func (pbi ProcPropertyBindInfoValue) MarshalBSON() ([]byte, error) {

	stdData := pbi.Std.toKV()
	if pbi.extra != nil {
		stdData = merge(stdData, pbi.extra.toMap())
	}
	return bson.Marshal(stdData)
}

func (pbi *ProcPropertyBindInfoValue) Validate() (string, error) {
	if err := pbi.Std.IP.Validate(); err != nil {
		return common.BKIP, err
	}
	if err := pbi.Std.Port.Validate(); err != nil {
		return common.BKPort, err
	}
	if err := pbi.Std.Protocol.Validate(); err != nil {
		return common.BKProtocol, err
	}
	if err := pbi.Std.Enable.Validate(); err != nil {
		return common.BKEnable, err
	}
	if pbi.extra != nil {
		return pbi.extra.Validate()
	}

	return "", nil

}

func (pbi stdProcPropertyBindInfoValue) toKV() map[string]interface{} {

	data := make(map[string]interface{}, 0)

	data["row_id"] = pbi.RowID
	data[common.BKIP] = pbi.IP
	data[common.BKPort] = pbi.Port
	data[common.BKProtocol] = pbi.Protocol
	data[common.BKEnable] = pbi.Enable
	return data
}

/*** ProcBindInfo 依赖的方法  ****/

func (pbi *ProcBindInfo) UnmarshalJSON(data []byte) error {
	err := defaultProcBindInfoHandle.UJSON(data, pbi)
	if err != nil {
		return err
	}
	return nil
}

func (pbi *ProcBindInfo) UnmarshalBSON(data []byte) error {
	err := defaultProcBindInfoHandle.UBSON(data, pbi)
	if err != nil {
		return err
	}
	return nil
}

func (pbi ProcBindInfo) MarshalJSON() ([]byte, error) {
	stdData := pbi.toKV()
	if pbi.extra != nil {
		stdData = merge(stdData, pbi.extra)
	}
	return json.Marshal(stdData)
}

func (pbi ProcBindInfo) MarshalBSON() ([]byte, error) {

	stdData := pbi.toKV()
	if pbi.extra != nil {
		stdData = merge(stdData, pbi.extra)
	}
	return bson.Marshal(stdData)
}

func (pbi ProcBindInfo) toKV() map[string]interface{} {
	data := make(map[string]interface{}, 0)

	data["template_row_id"] = pbi.Std.TemplateRowID
	data[common.BKIP] = pbi.Std.IP
	data[common.BKPort] = pbi.Std.Port
	data[common.BKProtocol] = pbi.Std.Protocol
	data[common.BKEnable] = pbi.Std.Enable
	return data
}

func merge(merge, merged map[string]interface{}) map[string]interface{} {
	if merge == nil {
		merge = make(map[string]interface{}, 0)
	}
	for key, val := range merged {
		merge[key] = val
	}

	return merge
}

/* 公开版本的进程bind 信息处理的方法 */

type openVersionProcBindInfo struct {
}

type openVersionPropertyBindInfo struct {
}

type processPropertyBindInfo struct {
}

func (ov *openVersionProcBindInfo) UJSON(data []byte, bindInfo *ProcBindInfo) error {
	if data == nil || len(data) == 0 {
		return nil
	}

	bindInfo.Std = &stdProcBindInfo{}
	// 公开版没有额外地址，直接解析到标准定义的结构中即可，不要就需要接到自定义结构中
	if err := json.Unmarshal(data, bindInfo.Std); err != nil {
		return err
	}
	return nil
}

func (ov *openVersionProcBindInfo) UBSON(data []byte, bindInfo *ProcBindInfo) error {
	if data == nil || len(data) == 0 {
		return nil
	}

	bindInfo.Std = &stdProcBindInfo{}
	// 公开版没有额外地址，直接解析到标准定义的结构中即可，不要就需要接到自定义结构中
	err := bson.Unmarshal(data, bindInfo.Std)
	return err
}

func (ov *openVersionPropertyBindInfo) UJSON(data []byte, bindInfo *ProcPropertyBindInfoValue) error {
	if data == nil || len(data) == 0 {
		return nil
	}

	// 公开版没有额外地址，直接解析到标准定义的结构中即可，不要就需要接到自定义结构中
	bindInfo.Std = &stdProcPropertyBindInfoValue{}
	err := json.Unmarshal(data, bindInfo.Std)
	if err != nil {
		return err
	}

	// 公开版没有额外数据无需再次解析，这里是做示例用的
	/*
		bindInfoExtra := &processPropertyBindInfo{}
		err := json.Unmarshal(data, &bindInfoExtra)
		if err != nil {
			return err
		}
		bindInfo.extra = bindInfoExtra
	*/
	return nil
}

func (ov *openVersionPropertyBindInfo) UBSON(data []byte, bindInfo *ProcPropertyBindInfoValue) error {
	if data == nil || len(data) == 0 {
		return nil
	}

	// 公开版没有额外地址，直接解析到标准定义的结构中即可，不要就需要接到自定义结构中
	bindInfo.Std = &stdProcPropertyBindInfoValue{}
	err := bson.Unmarshal(data, &bindInfo.Std)
	if err != nil {
		return err
	}

	// 公开版没有额外数据无需再次解析，这里是做示例用的
	/*
		bindInfoExtra := &processPropertyBindInfo{}
		err := bson.Unmarshal(data, &bindInfoExtra)
		if err != nil {
			return err
		}
		bindInfo.extra = bindInfoExtra
	*/

	return err
}

/*** 非标准属性需要实现的方法 ***/

func (ppbi *processPropertyBindInfo) Validate() (string, error) {
	// 公开版没有需要校验的额外字段
	return "", nil
}

func (ppbi *processPropertyBindInfo) ExtractChangeInfoBindInfo(i *ProcBindInfo) (map[string]interface{}, bool, bool) {
	// 公开版没有需要校验的额外字段
	return nil, false, false
}

func (ppbi *processPropertyBindInfo) ExtractInstanceUpdateData(extra map[string]interface{}) map[string]interface{} {
	// 公开版没有需要校验的额外字段
	return nil
}

func (ppbi *processPropertyBindInfo) toMap() map[string]interface{} {
	// 公开版没有需要校验的额外字段
	return nil
}
