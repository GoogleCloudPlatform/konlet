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

package command

import (
	"fmt"
	"os"
	"os/exec"
)

type Runner struct{}

// Wrap around os.exec.Command(...).CombinedOutput() to glue together output
// (STDERR+STDOUT) and execution error message upon failure.
//
// Convert the []byte output to string as well.
func (r Runner) Run(commandAndArgs ...string) (string, error) {
	if len(commandAndArgs) == 0 {
		return "", fmt.Errorf("No command provided.")
	}
	output, err := exec.Command(commandAndArgs[0], commandAndArgs[1:]...).CombinedOutput()
	outputString := string(output)
	if err != nil {
		errorString := fmt.Sprintf("%s", err)
		if outputString != "" {
			errorString = fmt.Sprintf("%s, details: %s", errorString, outputString)
		}
		return "", fmt.Errorf("Failed to execute command %s: %s", commandAndArgs, errorString)
	}
	return outputString, nil
}

func (r Runner) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (r Runner) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}
