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

package utils

import (
	"net/http"
	"fmt"
	"io/ioutil"
	"encoding/json"
	"log"
)

const AUTH_METADATA = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token"

type Token struct {
	AccessToken   string      `json:"access_token"`
	ExpiresIn     int         `json:"expires_in"`
	TokenType     string      `json:"token_type"`
}

type AuthProvider interface {
	RetrieveAuthToken() (string, error)
}

type ConstantTokenProvider struct {
	Token string
}

type ServiceAccountTokenProvider struct {
}

func (provider ServiceAccountTokenProvider) RetrieveAuthToken() (string, error) {
	log.Print("Downloading credentials for default VM service account from metadata server")
	client := &http.Client{}
	request, err := http.NewRequest("GET", AUTH_METADATA, nil)
	request.Header.Add("Metadata-Flavor", "Google")

	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("Metadata server responded with status %d", resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	res := Token{}
	err = json.Unmarshal(body, &res)
	if err != nil {
		return "", err
	}

	// TODO(gjaskiewicz): validate that AccessToken exists
	return res.AccessToken, nil
}

func (provider ConstantTokenProvider) RetrieveAuthToken() (string, error) {
	log.Print("Using user-provided credentials")
	return provider.Token, nil
}
