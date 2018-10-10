// Copyright 2018 Google Inc. All Rights Reserved.
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

package runtime

import (
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils"
	dockertypes "github.com/docker/engine-api/types"
	"testing"
)

func Test_containersStartedByKonlet_MatchesKonletContainers(t *testing.T) {
	containers := []dockertypes.Container{
		generateContainerStruct(t, "1", "/klt-container1-abcd"),
		generateContainerStruct(t, "2", "/container2"),
	}
	returned := containersStartedByKonlet(containers, "container2")
	expected := map[string]string{
		"1": "/klt-container1-abcd",
		"2": "/container2",
	}
	utils.AssertEqual(t, returned, expected, "")
}

func Test_containersStartedByKonlet_DoesNotMatchOtherContainers(t *testing.T) {
	containers := []dockertypes.Container{
		generateContainerStruct(t, "1", "/kltsadashdk"),
		generateContainerStruct(t, "2", "/user-container"),
	}
	returned := containersStartedByKonlet(containers, "container")
	expected := map[string]string{}
	utils.AssertEqual(t, returned, expected, "")
}

// generateContainerStruct is a utility function that fills some of the fields of
// dockertypes.Container for purposes of testing the functions in this package.
func generateContainerStruct(t *testing.T, id, name string) dockertypes.Container {
	t.Helper()
	return dockertypes.Container{
		ID:    id,
		Names: []string{name},
	}
}
