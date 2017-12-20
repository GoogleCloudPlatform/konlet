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
	"fmt"
	"log"
	"os/exec"
)

func printOutput(outs []byte) {
	if len(outs) > 0 {
		fmt.Printf("iptables command output: %s\n", string(outs))
	}
}

func OpenIptablesForProtocol(protocol string) error {
	log.Printf("Updating IPtables firewall rules - allowing %s traffic on all ports", protocol)
	// TODO: Make it use osCommandRunner.
	var cmd = exec.Command("iptables", "-A", "INPUT", "-p", protocol, "-j", "ACCEPT")
	var output, err = cmd.CombinedOutput()

	if err != nil {
		return err
	}
	// TODO(gjaskiewicz): check exit status and return an error
	printOutput(output)

	cmd = exec.Command("iptables", "-A", "FORWARD", "-p", protocol, "-j", "ACCEPT")
	output, err = cmd.CombinedOutput()

	if err != nil {
		return err
	}

	return nil
}

func OpenIptables() error {
	var err = OpenIptablesForProtocol("tcp")
	if err != nil {
		return err
	}
	err = OpenIptablesForProtocol("udp")
	if err != nil {
		return err
	}
	err = OpenIptablesForProtocol("icmp")
	if err != nil {
		return err
	}

	return nil
}
