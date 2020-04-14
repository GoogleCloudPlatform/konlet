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

import "strings"

// UseGcpTokenForImage returns true iff the provided image string points to
// a repository that uses a GCP token. Currently, that is:
//  - gcr.io
//  - pkg.dev
func UseGcpTokenForImage(image string) bool {
	parts := strings.SplitN(image, "/", 2)

	if len(parts) < 2 || len(parts[0]) == 0 {
		return false
	}

	hostname := strings.ToLower(parts[0])

	return hostname == "gcr.io" || strings.HasSuffix(hostname, ".gcr.io") || strings.HasSuffix(hostname, ".pkg.dev")
}
