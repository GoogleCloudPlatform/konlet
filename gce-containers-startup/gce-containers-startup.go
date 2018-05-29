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
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/command"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/metadata"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/runtime"
	"github.com/GoogleCloudPlatform/konlet/gce-containers-startup/utils"

	api "github.com/GoogleCloudPlatform/konlet/gce-containers-startup/types"
	yaml "gopkg.in/yaml.v2"
)

const METADATA_SERVER = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/gce-container-declaration"

var (
	tokenFlag       = flag.String("token", "", "what token to use")
	runDetachedFlag = flag.Bool("detached", true, "should we run container detached")
	showWelcomeFlag = flag.Bool("show-welcome", true, "should we show welcome on SSH to host OS")
	openIptables    = flag.Bool("open-iptables", true, "should we open IP Tables")
)

func main() {
	defer exitHandler()

	log.Printf("Starting Konlet container startup agent")
	flag.Parse()

	metadataProvider := metadata.DefaultProvider{}

	var authProvider utils.AuthProvider
	if *tokenFlag == "" {
		authProvider = utils.ServiceAccountTokenProvider{}
	} else {
		authProvider = utils.ConstantTokenProvider{Token: *tokenFlag}
	}

	runner, err := runtime.GetDefaultRunner(command.Runner{}, metadataProvider)
	if err != nil {
		log.Panicf("Failed to initialize Konlet: %v", err)
	}
	err = ExecStartup(metadataProvider, authProvider, runner, *openIptables)
	if err != nil {
		log.Panicf("Error: %v", err)
	}
}

func ExecStartup(metadataProvider metadata.Provider, authProvider utils.AuthProvider, runner *runtime.ContainerRunner, openIptables bool) error {
	body, err := metadataProvider.RetrieveManifest()
	if err != nil {
		return fmt.Errorf("Cannot load container declaration: %v", err)
	}

	declaration := api.ContainerSpec{}
	err = yaml.Unmarshal(body, &declaration)
	if err != nil {
		return fmt.Errorf("Cannot parse container declaration '%s': %v", body, err)
	}

	spec := declaration.Spec
	if len(spec.Containers) != 1 {
		return fmt.Errorf("Container declaration should include exactly 1 container, %d found", len(spec.Containers))
	}

	var auth = ""

	if utils.UseGcpTokenForImage(spec.Containers[0].Image) {
		auth, err = authProvider.RetrieveAuthToken()
		if err != nil {
			return fmt.Errorf("Cannot get auth token: %v", err)
		}
	} else {
		log.Printf("Non-GCR registry used - Konlet will use empty auth")
	}

	if openIptables {
		err = utils.OpenIptables()
		if err != nil {
			return fmt.Errorf("Cannot update IPtables: %v", err)
		}
	}

	log.Printf("Launching user container '%s'", spec.Containers[0].Image)
	err = runner.RunContainer(auth, spec, *runDetachedFlag)
	if err != nil {
		return fmt.Errorf("Failed to start container: %v", err)
	}

	return nil
}

func exitHandler() {
	err := recover()
	if *showWelcomeFlag {
		utils.WriteWelcomeScript(err)
	}
	if err != nil {
		os.Exit(1)
	}
}
