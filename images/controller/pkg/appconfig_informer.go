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

type AppConfigInformer struct {
	AddFunc    func(appConfig AppConfigObject)
	DeleteFunc func(appConfig AppConfigObject)
	UpdateFunc func(oldObj, newObj AppConfigObject)
	ApiVersion string
	Kind       string
	ApiGroup   string
	PodBrokerInformer
}

func NewAppConfigInformer(addFunc func(appConfig AppConfigObject), deleteFunc func(appConfig AppConfigObject), updateFunc func(oldObj, newObj AppConfigObject)) AppConfigInformer {
	resp := AppConfigInformer{
		ApiVersion: ApiVersion,
		Kind:       BrokerAppConfigKind,
		ApiGroup:   BrokerAppConfigApiGroup,
		AddFunc:    addFunc,
		DeleteFunc: deleteFunc,
		UpdateFunc: updateFunc,
	}
	return resp
}

// Interface function
func (pbi AppConfigInformer) makeObjectFromUnstructured(obj interface{}) (interface{}, error) {
	d := AppConfigObject{}
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
func (pbi AppConfigInformer) apiGroup() string {
	return pbi.ApiGroup
}

// Interface function
func (pbi AppConfigInformer) kind() string {
	return pbi.Kind
}

// Interface function
func (pbi AppConfigInformer) addFunc(obj interface{}) {
	pbi.AddFunc(obj.(AppConfigObject))
}

// Interface function
func (pbi AppConfigInformer) deleteFunc(obj interface{}) {
	pbi.DeleteFunc(obj.(AppConfigObject))
}

// Interface function
func (pbi AppConfigInformer) updateFunc(oldObj, newObj interface{}) {
	pbi.UpdateFunc(oldObj.(AppConfigObject), newObj.(AppConfigObject))
}
