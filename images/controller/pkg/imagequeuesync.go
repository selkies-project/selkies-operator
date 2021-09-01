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

import "sync"

// Synchronous slice of de-duplicated images to process
type ImageQueueSync struct {
	sync.Mutex
	ImageQueue []string
}

func NewImageQueueSync() *ImageQueueSync {
	iq := &ImageQueueSync{}
	iq.ImageQueue = make([]string, 0)
	return iq
}

func (iq *ImageQueueSync) Len() int {
	iq.Lock()
	defer iq.Unlock()
	return len(iq.ImageQueue)
}

func (iq *ImageQueueSync) Push(newImage string) {
	iq.Lock()
	defer iq.Unlock()
	for _, currImage := range iq.ImageQueue {
		if currImage == newImage {
			return
		}
	}
	iq.ImageQueue = append(iq.ImageQueue, newImage)
}

func (iq *ImageQueueSync) Pop() string {
	iq.Lock()
	defer iq.Unlock()
	if len(iq.ImageQueue) == 0 {
		return ""
	}
	v := iq.ImageQueue[0]
	iq.ImageQueue = iq.ImageQueue[1:]
	return v
}

func (iq *ImageQueueSync) Remove(image string) bool {
	iq.Lock()
	defer iq.Unlock()
	newQueue := make([]string, 0)
	found := false
	for _, currImage := range iq.ImageQueue {
		if currImage != image {
			newQueue = append(newQueue, currImage)
		} else {
			found = true
		}
	}
	return found
}

func (iq *ImageQueueSync) Contains(image string) bool {
	iq.Lock()
	defer iq.Unlock()
	for _, currImage := range iq.ImageQueue {
		if currImage == image {
			return true
		}
	}
	return false
}
