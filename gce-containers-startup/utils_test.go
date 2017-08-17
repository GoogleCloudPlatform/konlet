// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"testing"
	utils "github.com/konlet/utils"
	"fmt"
)

func TestDefaultRegistry_default(t *testing.T) {
	assertDefaultRegistry(t, "tomcat", true);
	assertDefaultRegistry(t, "tomcat:1.1", true);
}

func TestDefaultRegistry_dockerIo(t *testing.T) {
	assertDefaultRegistry(t, "docker.io/tomcat", true);
	assertDefaultRegistry(t, "index.docker.io/tomcat", true);
	assertDefaultRegistry(t, "docker.io/tomcat:1.1", true);
}

func TestDefaultRegistry_localRegistry(t *testing.T) {
	assertDefaultRegistry(t, "localhost.localdomain:5000/ubuntu", false);
}

func TestDefaultRegistry_gcr(t *testing.T) {
	assertDefaultRegistry(t, "gcr.io/google-containers/nginx", false);
	assertDefaultRegistry(t, "gcr.io/google-containers/nginx:1.2", false);
}

func assertDefaultRegistry(t *testing.T, image string, expectedDefault bool) {
	var errMsg = "default";
	if expectedDefault {
		errMsg = "non-default"
	}
	assertEqual(t,
		utils.IsDefaultRegistryMatch(image),
		expectedDefault,
		fmt.Sprintf("registry for %s was expected to be %s", image, errMsg));
}