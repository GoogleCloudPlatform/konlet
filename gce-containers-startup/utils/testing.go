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

package utils

import (
	"fmt"
	"reflect"
	"testing"
)

func AssertEqual(t *testing.T, a interface{}, b interface{}, message string) {
	if reflect.DeepEqual(a, b) {
		return
	}
	if len(message) == 0 {
		message = fmt.Sprintf("'%#v' != '%#v'", a, b)
	}
	t.Fatal(message)
}

func AssertNoError(t *testing.T, err error) {
	if err != nil {
		message := fmt.Sprintf("%v", err)
		t.Fatalf("Unexpected error '%s'", message)
	}
}

func AssertError(t *testing.T, err error, expected string) {
	if err == nil {
		t.Fatal("Exected error not to be null")
	}
	message := fmt.Sprintf("%v", err)
	if message != expected {
		t.Fatalf("Exected error to be '%s', but it was '%s'", expected, message)
	}
}

func AssertEmpty(t *testing.T, object interface{}, message string) {
	value := reflect.ValueOf(object)
	switch kind := value.Kind(); kind {
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice, reflect.String:
		if length := value.Len(); length != 0 {
			t.Fatalf("Expected an empty object, got a %s of length %d", kind.String(), length)
		}
	default:
		t.Fatalf("Expected a container-like object, got %s", kind.String())
	}
}
