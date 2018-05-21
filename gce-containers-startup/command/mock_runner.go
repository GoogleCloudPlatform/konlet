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

package command

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

type MockCommand struct {
	callCount      int
	returnedOutput string
	returnedError  error
}

type MockRunner struct {
	FailOnUnexpectedCalls bool
	commands              map[string]MockCommand
	statFiles             map[string]os.FileInfo
	expectedMkdirAlls     map[string]bool
	t                     *testing.T
}

type minimalFileInfo struct {
	name     string
	size     int64
	fileMode os.FileMode
	modTime  time.Time
}

func (f minimalFileInfo) Name() string {
	return f.name
}

func (f minimalFileInfo) Size() int64 {
	return f.size
}

func (f minimalFileInfo) Mode() os.FileMode {
	return f.fileMode
}

func (f minimalFileInfo) ModTime() time.Time {
	return f.modTime
}

func (f minimalFileInfo) IsDir() bool {
	return f.fileMode.IsDir()
}

func (f minimalFileInfo) Sys() interface{} {
	return nil
}

func NewMockRunner(t *testing.T) *MockRunner {
	return &MockRunner{FailOnUnexpectedCalls: true, commands: map[string]MockCommand{}, statFiles: map[string]os.FileInfo{}, expectedMkdirAlls: map[string]bool{}, t: t}
}

func (m *MockRunner) Run(commandAndArgs ...string) (string, error) {
	commandString := strings.Join(commandAndArgs, " ")
	if _, found := m.commands[commandString]; !found && m.FailOnUnexpectedCalls {
		m.t.Fatal(fmt.Sprintf("Unexpected os command called: %s", commandString))
	}
	m.incrementCallCount(commandString)
	return m.commands[commandString].returnedOutput, m.commands[commandString].returnedError
}

func (m *MockRunner) incrementCallCount(command string) {
	commandInfo := m.commands[command]
	commandInfo.callCount += 1
	m.commands[command] = commandInfo
}

func (m *MockRunner) MkdirAll(path string, perm os.FileMode) error {
	if _, found := m.expectedMkdirAlls[path]; !found && m.FailOnUnexpectedCalls {
		return fmt.Errorf("MkdirAll() called on unexpected path: %s", path)
	}
	m.expectedMkdirAlls[path] = true
	return nil
}

func (m *MockRunner) Stat(path string) (os.FileInfo, error) {
	fileInfo, found := m.statFiles[path]
	if !found && m.FailOnUnexpectedCalls {
		return minimalFileInfo{}, fmt.Errorf("MockRunner.Stat(): No such file or directory: %s", path)
	}
	return fileInfo, nil
}

func (m *MockRunner) OutputOnCall(commandAndArgs string, output string) {
	m.commands[commandAndArgs] = MockCommand{callCount: 0, returnedOutput: output, returnedError: nil}
}

func (m *MockRunner) ErrorOnCall(commandAndArgs string, err error) {
	m.commands[commandAndArgs] = MockCommand{callCount: 0, returnedOutput: "", returnedError: err}
}

func (m *MockRunner) RegisterMkdirAll(path string) {
	m.expectedMkdirAlls[path] = false
}

func (m *MockRunner) RegisterDeviceForStat(path string) {
	m.statFiles[path] = minimalFileInfo{name: path, fileMode: os.ModeDevice}
}

func (m *MockRunner) RegisterDirectoryForStat(path string) {
	m.statFiles[path] = minimalFileInfo{name: path, fileMode: os.ModeDir}
}

func (m *MockRunner) AssertCalled(commandAndArgs string) {
	command, found := m.commands[commandAndArgs]
	if !found || command.callCount == 0 {
		m.t.Fatal(fmt.Sprintf("Expected os command not called: %s", commandAndArgs))
	}
}

func (m *MockRunner) AssertAllCalled() {
	for commandAndArgs, command := range m.commands {
		if command.callCount == 0 {
			m.t.Fatal(fmt.Sprintf("Expected os command not called: %s", commandAndArgs))
		}
	}
	for mkdirAllPath, called := range m.expectedMkdirAlls {
		if !called {
			m.t.Fatal(fmt.Sprintf("Expected os.MkdirAll() not called: %s", mkdirAllPath))
		}
	}
}
