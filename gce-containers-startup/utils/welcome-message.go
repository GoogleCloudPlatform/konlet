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
)

const WARNING_SCRIPT = `#!/bin/bash
echo -e "\033[0;33m                                                             \033[0m"
echo -e "\033[0;33m                   >>\.                                      \033[0m"
echo -e "\033[0;33m                  /_  )'.                                    \033[0m"
echo -e "\033[0;33m                 /  _)'^)'.   ___.---. _                     \033[0m"
echo -e "\033[0;33m                (_,' \  '^-)\"\"        '.--__               \033[0m"
echo -e "\033[0;33m                      \                |'-##-.               \033[0m"
echo -e "\033[0;33m                   ____)               /   \#^\              \033[0m"
echo -e "\033[0;33m                  (  ___/--._____.-\  (_    WW\              \033[0m"
echo -e "\033[0;33m                   ^_\_             \ \_^-._                 \033[0m"
echo -e "\033[0;33m                   //._]             )/ --,_\                \033[0m"
echo -e "\033[0;33m                  /_>               |_>                      \033[0m"
echo -e "\033[0;33m                                                             \033[0m"
echo -e "\033[0;33m  ########################[ Welcome ]########################\033[0m"
echo -e "\033[0;33m  #  You have logged in to the guest OS.                    #\033[0m"
echo -e "\033[0;33m  #  To access your containers use 'docker attach' command  #\033[0m"
echo -e "\033[0;33m  ###########################################################\033[0m"
echo -e "\033[0;33m                                                             \033[0m"`

const SCRIPT_DIR = "/host/etc/profile.d"

func WriteWelcomeScript() error {
	data := []byte(WARNING_SCRIPT)
	err := ioutil.WriteFile(SCRIPT_DIR + "/gce-containers-welcome.sh", data, 0755)
	if err != nil {
		return err
	}
	return nil
}
