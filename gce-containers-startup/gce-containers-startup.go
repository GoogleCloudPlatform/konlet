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
	"os"
	"fmt"
	"io/ioutil"
	"net/http"
	"log"
	"flag"

	yaml "gopkg.in/yaml.v2"
	utils "github.com/konlet/utils"
	api "github.com/konlet/types"
)

const METADATA_SERVER = "http://metadata.google.internal/computeMetadata/v1/instance/attributes/gce-container-declaration"

var (
	tokenFlag = flag.String("token", "", "what token to use")
	manifestUrlFlag = flag.String("manifest-url", "", "URL for loading container manifest")
	runDetachedFlag = flag.Bool("detached", true, "should we run container detached")
	showWarningFlag = flag.Bool("show-warning", true, "should we show warning on SSH to host OS")
	openIptables = flag.Bool("open-iptables", true, "should we open IP Tables")
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
		fmt.Fprintf(os.Stderr, "--- Using URL for manifest: %s \n", *manifestUrlFlag)
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
		return nil, fmt.Errorf("Can't read metadata: (%s).\n", resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func main() {
	fmt.Fprintf(os.Stderr, "--- Starting Konlet\n")
	flag.Parse()

	var manifestProvider ManifestProvider
	manifestProvider = DefaultManifestProvider{}

	var authProvider utils.AuthProvider
	if *tokenFlag == "" {
		authProvider = utils.ServiceAccountTokenProvider{}
	} else {
		authProvider = utils.ConstantTokenProvider{Token: *tokenFlag, }
	}

	runner, err := utils.GetDefaultRunner()
	if err != nil {
		log.Fatalf("failed to create container runner: %v", err)
		return
	}

	err = ExecStartup(manifestProvider, authProvider, runner, *openIptables)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func ExecStartup(manifestProvider ManifestProvider, authProvider utils.AuthProvider, runner *utils.ContainerRunner, openIptables bool) error {
	body, err := manifestProvider.RetrieveManifest()
	if err != nil {
		return fmt.Errorf("Cannot load container manifest: %v", err)
	}

	declaration := api.ContainerSpec{}
	err = yaml.Unmarshal(body, &declaration)
	if err != nil {
		return fmt.Errorf("Cannot parse container manifest '%s': %v", body, err)
	}

	spec := declaration.Spec
	if len(spec.Containers) != 1 {
		return fmt.Errorf("There could be exactly 1 container in specification")
	}

	log.Print("-- Running docker")
	var auth string
	auth, err = authProvider.RetrieveAuthToken()
	if err != nil {
		return fmt.Errorf("Could not get GCR auth token: %v", err)
	}

	if openIptables {
		err = utils.OpenIptables()
		if err != nil {
			return fmt.Errorf("Could not open IP tables: %v", err)
		}
	}

	err = runner.RunContainer(auth, spec, *runDetachedFlag)
	if err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	if *showWarningFlag {
		log.Print("-- Saving warning script to profile.d")
		utils.WriteWarningScript()
	}

	return nil
}
