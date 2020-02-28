/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package types

type Yaml struct {
	Local []Local `yaml:"local"`
}

type Local struct {
	DataId string `yaml:"dataid"`
	Paths []string `yaml:"paths"`
	ToJson bool `yaml:"to_json"`
	ExtMeta map[string]string `yaml:"ext_meta"`

	//stdout dataid
	StdoutDataid string `yaml:"-"`
	//nonstandard log dataid
	NonstandardDataid string `yaml:"-"`
	//nonstandard paths
	NonstandardPaths string `yaml:"-"`
}
