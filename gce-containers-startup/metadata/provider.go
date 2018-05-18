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

package metadata

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

const (
	METADATA_SERVER_URL            = "http://metadata.google.internal/computeMetadata/v1/"
	CONTAINER_DECLARATION_METADATA = "instance/attributes/gce-container-declaration"
	DISKS_METADATA_WITH_RECURSION  = "instance/disks/?recursive=true"
)

type Provider interface {
	RetrieveManifest() ([]byte, error)
	RetrieveDisksMetadataAsJson() ([]byte, error)
}

type DefaultProvider struct {
}

func (provider DefaultProvider) queryMetadataServer(partialUrl string) ([]byte, error) {
	client := &http.Client{}
	metadataPath := METADATA_SERVER_URL + partialUrl

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
		return nil, fmt.Errorf("Failed metadata request: (%s).\n", resp.Status)
	}

	return ioutil.ReadAll(resp.Body)
}

func (provider DefaultProvider) RetrieveManifest() ([]byte, error) {
	return provider.queryMetadataServer(CONTAINER_DECLARATION_METADATA)
}

func (provider DefaultProvider) RetrieveDisksMetadataAsJson() ([]byte, error) {
	return provider.queryMetadataServer(DISKS_METADATA_WITH_RECURSION)
}
