/*
 Copyright 2021 The Selkies Authors. All rights reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package pod_broker

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type AppUserConfigInformer struct {
	AddFunc    func(appConfig AppUserConfigObject)
	DeleteFunc func(appConfig AppUserConfigObject)
	UpdateFunc func(oldObj, newObj AppUserConfigObject)
	ApiVersion string
	Kind       string
	ApiGroup   string
}

func NewAppUserConfigInformer(addFunc func(appConfig AppUserConfigObject), deleteFunc func(appConfig AppUserConfigObject), updateFunc func(oldObj, newObj AppUserConfigObject)) AppUserConfigInformer {
	resp := AppUserConfigInformer{
		ApiVersion: ApiVersion,
		Kind:       BrokerAppUserConfigKind,
		ApiGroup:   BrokerAppUserConfigApiGroup,
		AddFunc:    addFunc,
		DeleteFunc: deleteFunc,
		UpdateFunc: updateFunc,
	}
	return resp
}

// Interface function
func (pbi AppUserConfigInformer) makeObjectFromUnstructured(obj interface{}) (interface{}, error) {
	d := AppUserConfigObject{}
	err := runtime.DefaultUnstructuredConverter.
		FromUnstructured(obj.(*unstructured.Unstructured).UnstructuredContent(), &d)
	if err != nil {
		return d, err
	}
	d.ApiVersion = pbi.ApiVersion
	d.Kind = pbi.Kind
	return d, nil
}

// Interface function
func (pbi AppUserConfigInformer) apiGroup() string {
	return pbi.ApiGroup
}

// Interface function
func (pbi AppUserConfigInformer) kind() string {
	return pbi.Kind
}

// Interface function
func (pbi AppUserConfigInformer) addFunc(obj interface{}) {
	pbi.AddFunc(obj.(AppUserConfigObject))
}

// Interface function
func (pbi AppUserConfigInformer) deleteFunc(obj interface{}) {
	pbi.DeleteFunc(obj.(AppUserConfigObject))
}

// Interface function
func (pbi AppUserConfigInformer) updateFunc(oldObj, newObj interface{}) {
	pbi.UpdateFunc(oldObj.(AppUserConfigObject), newObj.(AppUserConfigObject))
}
