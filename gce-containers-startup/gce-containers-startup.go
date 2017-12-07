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
	"io/ioutil"
	"log"
	"net/http"

	api "github.com/konlet/types"
	utils "github.com/konlet/utils"
	yaml "gopkg.in/yaml.v2"
)

const METADATA_SERVER = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/gce-container-declaration"

var (
	tokenFlag       = flag.String("token", "", "what token to use")
	manifestUrlFlag = flag.String("manifest-url", "", "URL for loading container manifest")
	runDetachedFlag = flag.Bool("detached", true, "should we run container detached")
	showWelcomeFlag = flag.Bool("show-welcome", true, "should we show welcome on SSH to host OS")
	openIptables    = flag.Bool("open-iptables", true, "should we open IP Tables")
)

type ManifestProvider interface {
	RetrieveManifest() ([]byte, error)
}

type DefaultManifestProvider struct {
}

func (provider DefaultManifestProvider) RetrieveManifest() ([]byte, error) {
	client := &http.Client{}
	metadataPath := METADATA_SERVER
	if *manifestUrlFlag != "" {
		log.Printf("Retrieving container declaration from '%s' \n", *manifestUrlFlag)
		metadataPath = *manifestUrlFlag
	}

	request, err := http.NewRequest("GET", metadataPath, nil)
	if err != nil {
		return nil, err
	}

	request.Header.Add("Metadata-Flavor", "Google")
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Cannot read container declaration: (%s).\n", resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func main() {
	log.Printf("Starting Konlet container startup agent")
	flag.Parse()

	var manifestProvider ManifestProvider
	manifestProvider = DefaultManifestProvider{}

	var authProvider utils.AuthProvider
	if *tokenFlag == "" {
		authProvider = utils.ServiceAccountTokenProvider{}
	} else {
		authProvider = utils.ConstantTokenProvider{Token: *tokenFlag}
	}

	runner, err := utils.GetDefaultRunner()
	if err != nil {
		log.Fatalf("Failed to initialize Konlet: %v", err)
		return
	}
	err = ExecStartup(manifestProvider, authProvider, runner, utils.RealOsCommandRunner{}, *openIptables)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func ExecStartup(manifestProvider ManifestProvider, authProvider utils.AuthProvider, runner *utils.ContainerRunner, osCommandRunner utils.OsCommandRunnerInterface, openIptables bool) error {
	utils.OsCommandRunner = osCommandRunner
	body, err := manifestProvider.RetrieveManifest()
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

	if !utils.IsDefaultRegistryMatch(spec.Containers[0].Image) {
		auth, err = authProvider.RetrieveAuthToken()
		if err != nil {
			return fmt.Errorf("Cannot get auth token: %v", err)
		}
	} else {
		log.Printf("Default registry used - Konlet will use empty auth")
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

	if *showWelcomeFlag {
		log.Print("Saving welcome script to profile.d")
		utils.WriteWelcomeScript()
	}

	return nil
}
