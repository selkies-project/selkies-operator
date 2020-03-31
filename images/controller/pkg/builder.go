/*
 Copyright 2019 Google Inc. All rights reserved.

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
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

func BuildDeploy(brokerCommonBaseDir, srcDir, destDir string, data *UserPodData) error {
	// Clear out destdir before building it.
	os.RemoveAll(destDir)

	err := os.MkdirAll(destDir, os.ModePerm)
	if err != nil {
		return err
	}

	// Search buildsrc and subdirectories for files.
	searchPaths := []string{
		srcDir,
		path.Join(srcDir, "*"),
		brokerCommonBaseDir,
		path.Join(brokerCommonBaseDir, "*"),
	}

	// files ending with *.tmpl will be passed through the gotemplate engine.
	var templateFiles []string

	// Files ending in .yaml will be copied to the build dir as-is.
	var copyFiles []string

	// Files prefixed with "resource-" will be added to the list of kustomize resources.
	var resourceFiles []string

	// Files prefixed with "patch-" will be added to the list of kustomize patches.
	var patchFiles []string

	// Files prefixed with "jsonpatch-svc-" will be added to the list of kustomize patchesJson6902 patches for the service.
	var jsonPatchServiceFiles []string

	// Files prefixed with "jsonpatch-virtualservice-" will be added to the list of kustomize patchesJson6902 patches for the virtualservice.
	var jsonPatchVirtualServiceFiles []string

	// Files prefixed with "jsonpatch-deploy-" will be added to the list of kustomize patchesJson6902 patches for the statefulset.
	var jsonPatchDeployFiles []string

	var foundFiles []string
	for _, searchPath := range searchPaths {
		foundFiles, err = filepath.Glob(path.Join(searchPath, "*.tmpl"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for templateFiles: %v", err)
		}
		templateFiles = append(templateFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "*.yaml"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for copyFiles: %v", err)
		}
		copyFiles = append(copyFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "resource-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for resourceFiles: %v", err)
		}
		resourceFiles = append(resourceFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "patch-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for patchFiles: %v", err)
		}
		patchFiles = append(patchFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "jsonpatch-service-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for jsonPatchServiceFiles: %v", err)
		}
		jsonPatchServiceFiles = append(jsonPatchServiceFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "jsonpatch-virtualservice-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for jsonPatchVirtualServiceFiles: %v", err)
		}
		jsonPatchVirtualServiceFiles = append(jsonPatchVirtualServiceFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "jsonpatch-deploy-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for jsonPatchDeployFiles: %v", err)
		}
		jsonPatchDeployFiles = append(jsonPatchDeployFiles, foundFiles...)
	}

	// Add resource files to UserPodData
	for _, resource := range resourceFiles {
		data.Resources = append(data.Resources, strings.ReplaceAll(path.Base(resource), ".tmpl", ""))
	}

	// Add patch files to UserPodData
	for _, patch := range patchFiles {
		data.Patches = append(data.Patches, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
	}

	// Add service jsonpatch files to UserPodData
	for _, patch := range jsonPatchServiceFiles {
		data.JSONPatchesService = append(data.JSONPatchesService, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
	}

	// Add virtual service jsonpatch files to UserPodData
	for _, patch := range jsonPatchVirtualServiceFiles {
		data.JSONPatchesVirtualService = append(data.JSONPatchesVirtualService, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
	}

	// Add deploy jsonpatch files to UserPodData
	for _, patch := range jsonPatchDeployFiles {
		data.JSONPatchesDeploy = append(data.JSONPatchesDeploy, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
	}

	// Copy build files
	for i := range copyFiles {
		if err := CopyFile(copyFiles[i], destDir); err != nil {
			return fmt.Errorf("error copying file: %s: %v", copyFiles[i], err)
		}
	}

	// Write kustomization.
	for i := range templateFiles {
		if err := TemplateFile(templateFiles[i], destDir, data); err != nil {
			return fmt.Errorf("error templating file: %s: %v", templateFiles[i], err)
		}
	}

	return nil
}

func TemplateFile(templatePath, destDir string, data *UserPodData) error {
	base := path.Base(templatePath)
	t, err := template.New(base).Funcs(sprig.TxtFuncMap()).ParseFiles(templatePath)
	if err != nil {
		return err
	}
	dest, _ := os.Create(strings.ReplaceAll(path.Join(destDir, base), ".tmpl", ""))
	if err != nil {
		return err
	}
	return t.Execute(dest, data)
}
