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
    "io/ioutil"
    "log"
)

const WELCOME_SCRIPT_ON_SUCCESS = `#!/bin/bash
echo -e "\033[0;33m  ########################[ Welcome ]########################\033[0m"
echo -e "\033[0;33m  #  You have logged in to the guest OS.                    #\033[0m"
echo -e "\033[0;33m  #  To access your containers use 'docker attach' command  #\033[0m"
echo -e "\033[0;33m  ###########################################################\033[0m"
echo -e "\033[0;33m                                                             \033[0m"
`

const WELCOME_SCRIPT_ON_FAILURE = `#!/bin/bash
echo -e "\033[0;31m  #########################[ Error ]#########################\033[0m"
echo -e "\033[0;31m  #  The startup agent encountered errors. Your container   #\033[0m"
echo -e "\033[0;31m  #  was not started. To inspect the agent's logs use       #\033[0m"
echo -e "\033[0;31m  #  'sudo journalctl -u konlet-startup' command.           #\033[0m"
echo -e "\033[0;31m  ###########################################################\033[0m"
echo -e "\033[0;31m                                                             \033[0m"
`
const SCRIPT_DIR = "/host/etc/profile.d"

func WriteWelcomeScript(startupErr interface{}) error {
  var script string
  if startupErr == nil {
    script = WELCOME_SCRIPT_ON_SUCCESS
  } else {
    script = WELCOME_SCRIPT_ON_FAILURE
  }
  data := []byte(script)
  writeErr := ioutil.WriteFile(SCRIPT_DIR + "/gce-containers-welcome.sh", data, 0755)
  if writeErr != nil {
    log.Print("Failed to save welcome script to profile.d")
    return writeErr
  } else {
    log.Print("Saving welcome script to profile.d")
    return nil
  }
}
