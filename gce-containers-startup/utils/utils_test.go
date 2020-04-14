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
	"errors"
	"fmt"
	"os"
	"testing"
)

func TestDefaultRegistry_default(t *testing.T) {
	assertRegistryGetsToken(t, "tomcat", false)
	assertRegistryGetsToken(t, "tomcat:1.1", false)
}

func TestDefaultRegistry_dockerIo(t *testing.T) {
	assertRegistryGetsToken(t, "docker.io/tomcat", false)
	assertRegistryGetsToken(t, "index.docker.io/tomcat", false)
	assertRegistryGetsToken(t, "docker.io/tomcat:1.1", false)
}

func TestDefaultRegistry_localRegistry(t *testing.T) {
	assertRegistryGetsToken(t, "localhost.localdomain:5000/ubuntu", false)
}

func TestDefaultRegistry_gcr(t *testing.T) {
	assertRegistryGetsToken(t, "gcr.io/google-containers/nginx", true)
	assertRegistryGetsToken(t, "gcr.io/google-containers/nginx:1.2", true)
	assertRegistryGetsToken(t, "asia.gcr.io/other-containers/busybox", true)
	assertRegistryGetsToken(t, "oh.my.this-is-not-gcr.io/other-containers/busybox", false)
}

func TestDefaultRegistry_ar(t *testing.T) {
	assertRegistryGetsToken(t, "us-docker.pkg.dev/project/google-containers/nginx", true)
	assertRegistryGetsToken(t, "us-west1-docker.pkg.dev/project/google-containers/nginx:1.2", true)
	assertRegistryGetsToken(t, "australia-southeast1-docker.pkg.dev/project/other-containers/busybox", true)
	assertRegistryGetsToken(t, "not-docker-pkg.dev/project/other-containers/busybox", false)
}

func assertRegistryGetsToken(t *testing.T, image string, expectedToken bool) {
	AssertEqual(t,
		UseGcpTokenForImage(image),
		expectedToken,
		fmt.Sprintf("registry for %s: unexpected use token: %t", image, !expectedToken))
}

func TestSelectScriptBasedOnStartupResult_ReturnsFailureScriptOnStartupFailure(t *testing.T) {
	actualScript := selectScriptBasedOnStartupResult(errors.New("some error"))
	AssertEqual(t,
		actualScript,
		WELCOME_SCRIPT_ON_FAILURE,
		fmt.Sprintf("Wrong startup script. Got:\n%v\nExpected:\n%v", actualScript, WELCOME_SCRIPT_ON_FAILURE))
}

func TestSelectScriptBasedOnStartupResult_ReturnsSuccessScriptOnStartupSuccess(t *testing.T) {
	actualScript := selectScriptBasedOnStartupResult(nil)
	AssertEqual(t,
		actualScript,
		WELCOME_SCRIPT_ON_SUCCESS,
		fmt.Sprintf("Wrong startup script. Got:\n%v\nExpected:\n%v", actualScript, WELCOME_SCRIPT_ON_SUCCESS))
}

func TestSaveScriptToFile_CallsFileWriterWithCorrectParameters(t *testing.T) {
	writer := &MockFileWriter{}
	dummyScript := "test script"
	saveScriptToFile(dummyScript, writer, &MockLogger{})
	AssertEqual(t, writer.filename, SCRIPT_PATH, "")
	AssertEqual(t, writer.data, []byte(dummyScript), "")
	AssertEqual(t, writer.perm, os.FileMode(0755), "")
}

func TestSaveScriptToFile_PrintsLogAndReturnsError(t *testing.T) {
	tests := []struct {
		err error
		log string
	}{
		{errors.New("some error"), "Failed to save welcome script to profile.d"},
		{nil, "Saving welcome script to profile.d"},
	}

	for _, test := range tests {
		logger := &MockLogger{}
		actualError := saveScriptToFile("test script", &MockFileWriter{returnValue: test.err}, logger)
		AssertEqual(t, actualError, test.err, "")
		AssertEqual(t, logger.message, test.log, "")
	}
}

type MockFileWriter struct {
	returnValue error
	filename    string
	data        []byte
	perm        os.FileMode
}

func (writer *MockFileWriter) WriteFile(filename string, data []byte, perm os.FileMode) error {
	writer.filename = filename
	writer.data = data
	writer.perm = perm
	return writer.returnValue
}

type MockLogger struct {
	message string
}

func (logger *MockLogger) Print(v ...interface{}) {
	logger.message = fmt.Sprint(v...)
}
