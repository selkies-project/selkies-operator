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
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type PodBrokerInformer interface {
	makeObjectFromUnstructured(obj interface{}) (interface{}, error)
	addFunc(obj interface{})
	deleteFunc(obj interface{})
	updateFunc(oldObj interface{}, newObj interface{})
	apiGroup() string
	kind() string
}

type PodBrokerInformerOpts struct {
	ResyncDuration time.Duration
	ClientConfig   *rest.Config
}

func RunPodBrokerInformer(pbi PodBrokerInformer, stopCh <-chan struct{}, opts *PodBrokerInformerOpts) error {
	log.Printf("starting informer for CRD %s", pbi.kind())
	handlers := cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			d, err := pbi.makeObjectFromUnstructured(obj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			//log.Printf("Saw new %s: %s", pbi.GetKind(), d.Metadata.Name)
			pbi.addFunc(d)
		},
		DeleteFunc: func(obj interface{}) {
			d, err := pbi.makeObjectFromUnstructured(obj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			//log.Printf("Saw deletion of %s: %s", pbi.GetKind(), d.Metadata.Name)
			pbi.deleteFunc(d)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			newData, err := pbi.makeObjectFromUnstructured(newObj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			oldData, err := pbi.makeObjectFromUnstructured(oldObj)
			if err != nil {
				fmt.Printf("could not convert obj: %v", err)
				return
			}
			//log.Printf("Saw update for %s: %s", pbi.GetKind(), d.Metadata.Name)
			pbi.updateFunc(oldData, newData)
		},
	}
	myinformer, err := getDynamicInformer(opts.ClientConfig, pbi.apiGroup(), opts.ResyncDuration)
	if err != nil {
		return err
	}
	s := myinformer.Informer()
	s.AddEventHandler(handlers)
	s.Run(stopCh)

	log.Printf("informer shutting down")

	return nil
}

func getDynamicInformer(cfg *rest.Config, resourceType string, resyncDuration time.Duration) (informers.GenericInformer, error) {
	// Grab a dynamic interface that we can create informers from
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	// Create a factory object that can generate informers for resource types
	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(dc, resyncDuration, corev1.NamespaceAll, nil)
	// "GroupVersionResource" to say what to watch e.g. "deployments.v1.apps" or "seldondeployments.v1.machinelearning.seldon.io"
	gvr, _ := schema.ParseResourceArg(resourceType)
	// Finally, create our informer for deployments!
	informer := factory.ForResource(*gvr)
	return informer, nil
}
