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
	"bufio"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
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
		path.Join(srcDir, "*", "*"),
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

	// Files prefixed with "jsonpatch-ns-" will e added to the list of kustomize patchesJson6902 patches for the user namespace.
	var jsonPatchNamespaceFiles []string

	// Files prefixed with "jsonpatch-sa-" will e added to the list of kustomize patchesJson6902 patches for the user service account.
	var jsonPatchServiceAccountFiles []string

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

		foundFiles, err = filepath.Glob(path.Join(searchPath, "jsonpatch-ns-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for jsonPatchNamespaceFiles: %v", err)
		}
		jsonPatchNamespaceFiles = append(jsonPatchNamespaceFiles, foundFiles...)

		foundFiles, err = filepath.Glob(path.Join(searchPath, "jsonpatch-sa-*"))
		if err != nil {
			return fmt.Errorf("bad glob pattern for jsonPatchServiceAccountFiles: %v", err)
		}
		jsonPatchServiceAccountFiles = append(jsonPatchServiceAccountFiles, foundFiles...)
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

	// Add namespace jsonpatch files to UserPodData
	for _, patch := range jsonPatchNamespaceFiles {
		data.JSONPatchesNamespace = append(data.JSONPatchesNamespace, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
	}

	// Add service account jsonpatch files to UserPodData
	for _, patch := range jsonPatchServiceAccountFiles {
		data.JSONPatchesServiceAccount = append(data.JSONPatchesServiceAccount, strings.ReplaceAll(path.Base(patch), ".tmpl", ""))
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

func ChecksumDeploy(srcDir string) (string, error) {
	res := ""

	fileMap, err := MD5All(srcDir)
	if err != nil {
		return res, err
	}

	keys := make([]string, 0)
	for k, _ := range fileMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	hasher := md5.New()
	for _, k := range keys {
		data := fileMap[k]
		hasher.Write(data[:])
	}

	res = hex.EncodeToString(hasher.Sum(nil))

	return res, nil
}

// Uses kustomize to build the bundle and returns the list of object types found.
func GetObjectTypes(srcDir string) ([]string, error) {
	resp := make([]string, 0)

	cmd := exec.Command("sh", "-o", "pipefail", "-c", fmt.Sprintf("kustomize build %s", srcDir))
	cmd.Dir = srcDir
	stdoutStderr, err := cmd.CombinedOutput()
	if err != nil {
		return resp, fmt.Errorf("error running kustomize build on directory %s: %v\n%s", srcDir, err, stdoutStderr)
	}

	kindMap := make(map[string]int, 0)
	scanner := bufio.NewScanner(bytes.NewReader(stdoutStderr))
	kindPat := regexp.MustCompile(`^kind: (?P<kind>.*)`)
	for scanner.Scan() {
		line := scanner.Text()
		match := kindPat.FindStringSubmatch(line)
		if len(match) > 1 {
			kind := match[1]
			kindMap[kind] += 1
		}
	}

	for k := range kindMap {
		resp = append(resp, k)
	}
	sort.Strings(resp)

	return resp, nil
}

func GenerateObjectTypePatch(srcDir string) error {
	destFile := path.Join(srcDir, JSONPatchObjectTypes)
	if _, err := os.Stat(destFile); os.IsNotExist(err) {
		return fmt.Errorf("dest template file not found: %s", destFile)
	}
	foundTypes, err := GetObjectTypes(srcDir)
	if err != nil {
		return err
	}
	objects := strings.Join(foundTypes, ",")

	data := fmt.Sprintf(`- op: add
  path: "/spec/template/metadata/annotations/app.broker~1last-applied-object-types"
  value: %s
`, objects)
	return ioutil.WriteFile(destFile, []byte(data), 0644)
}

// MD5All reads all the files in the file tree rooted at root and returns a map
// from file path to the MD5 sum of the file's contents.  If the directory walk
// fails or any read operation fails, MD5All returns an error.
// https://stackoverflow.com/a/50134601
func MD5All(root string) (map[string][md5.Size]byte, error) {
	m := make(map[string][md5.Size]byte)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		m[path] = md5.Sum(data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}
