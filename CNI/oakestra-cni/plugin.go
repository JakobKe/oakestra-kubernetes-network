// Copyright (c) 2015-2021 Tigera, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cniv1 "github.com/containernetworking/cni/pkg/types/100"
	cniSpecVersion "github.com/containernetworking/cni/pkg/version"
	"github.com/sirupsen/logrus"
)

func init() {
	// This ensures that main runs only on main thread (thread group leader).
	// since namespace ops (unshare, setns) are done for a single thread, we
	// must ensure that the goroutine does not jump from OS thread to thread
	runtime.LockOSThread()

	// Pfad zur Logdatei
	logFilePath := "/home/ubuntu/cni_log.txt" // TODO

	// Öffne oder erstelle die Logdatei
	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Fehler beim Öffnen der Logdatei: %v\n", err)
		os.Exit(1)
	}

	// Setze die Ausgabe für das Logging auf die Logdatei
	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func extractServiceNameAndInstanceNumber(input string) (string, int) {
	log.Println(input)
	parts := strings.Split(input, ".")
	// Servicename befindet sich an Index 2 und Instanznummer an Index 4
	serviceName := strings.Join(parts[:len(parts)-1], ".")
	instanceNumber, err := strconv.Atoi(parts[4]) // TODO Hier brauche ich ungbedingt ein error Hanlding, wenn der Name nicht passt!
	if err != nil {
		log.Printf("Fehler beim Konvertieren der Instanznummer: %v", err)
		return serviceName, 0 // Rückgabe eines Standardwerts im Fehlerfall
	}
	return serviceName, instanceNumber
}

func extractPodName(input string) string {
	// Trennen Sie den Eingabestring anhand des Semikolons
	parts := strings.Split(input, ";")

	// Durchsuchen Sie die Teile nach dem Token "K8S_POD_NAME="
	for _, part := range parts {
		if strings.HasPrefix(part, "K8S_POD_NAME=") {
			// Extrahieren Sie den Namen des Pods, indem Sie das Präfix "K8S_POD_NAME=" entfernen
			return strings.TrimPrefix(part, "K8S_POD_NAME=")
		}
	}

	// Rückgabe eines leeren Strings, wenn der Podname nicht gefunden wird
	return ""
}

// func extractPidNumber(input string) int {
// 	log.Printf("PID STRING: %s", input)
// 	parts := strings.Split(input, "/")
// 	// if len(parts) >= 2 {
// 	// 	pid, err := strconv.Atoi(parts[2])
// 	// 	if err != nil {
// 	// 		log.Printf("Fehler beim Konvertieren der PID: %v", err)
// 	// 		//return 0 // Rückgabe eines Standardwerts im Fehlerfall
// 	// 	}
// 	// 	return pid
// 	// }

// 	// SOME TESTS

// 	namespaceID := parts[len(parts)-1]

// 	log.Println(namespaceID)

// 	log.Println("START PAUSE")

// 	time.Sleep(5000 * time.Second)

// 	// Command to execute
// 	cmd := exec.Command("ip", "netns", "pids", namespaceID)

// 	// Execute the command
// 	var out bytes.Buffer
// 	cmd.Stdout = &out
// 	err := cmd.Run()
// 	if err != nil {
// 		log.Printf("Error executing command: %v", err)
// 		return 0
// 	}

// 	// Print the output
// 	log.Printf("OUTPUT: %s", out.String())

// 	return 0 // Rückgabe eines Standardwerts, wenn der Eingabestring nicht dem erwarteten Format entspricht
// }

func cmdAdd(args *skel.CmdArgs) (err error) {

	conf := types.NetConf{}
	conf.CNIVersion = "0.3.0"

	var result cniv1.Result

	log.Println(args)

	//time.Sleep(5 * time.Minute)

	log.Printf("ADD COMMAND")
	podName := extractPodName(args.Args)
	serviceName, instanceNumber := extractServiceNameAndInstanceNumber(podName)
	// pid := extractPidNumber(args.Netns)

	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Oakestra CNI panicked during ADD: %s\nStack trace:\n%s", e, string(debug.Stack()))
			if err != nil {
				// If we're recovering and there was also an error, then we need to
				// present both.
				msg = fmt.Sprintf("%s: error=%s", msg, err)
			}
			err = fmt.Errorf(msg)
		}
		if err != nil {
			logrus.WithError(err).Error("Final result of CNI ADD was an error.")
		}
	}()

	parts := strings.Split(args.Netns, "/")
	networkNamespace := parts[len(parts)-1]
	log.Printf("NetworkNamespace: %s", networkNamespace)
	requestBody := connectNetworkRequest{
		NetworkNamespace: networkNamespace,
		Servicename:      serviceName,    // Wird anscheinend der komplexe Name erwartet?
		Instancenumber:   instanceNumber, // TODO jetzt ist die Information hier theoretisch doppelt drin.
		PodName:          podName,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Fehler beim Erstellen des JSON: %v", err)
		return err
	}

	url := "http://localhost:6000/container/deploy"

	// HTTP-POST-Anfrage an den Server senden
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Fehler beim Senden der Anfrage: %v", err)
	}
	defer resp.Body.Close()

	// Antwort des Servers in den Log drucken
	log.Printf("Antwort des Servers: %v", resp.Status)

	// TODO I need to return the interface name

	// interfaceName := ""

	// result.Interfaces = append(result.Interfaces, &cniv1.Interface{
	// 	Name: interfaceName})

	log.Println("End of ADD")

	err = cnitypes.PrintResult(&result, conf.CNIVersion)

	return
}

func cmdDel(args *skel.CmdArgs) (err error) {
	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Oakestra CNI panicked during DEL: %s\nStack trace:\n%s", e, string(debug.Stack()))
			if err != nil {
				// If we're recovering and there was also an error, then we need to
				// present both.
				msg = fmt.Sprintf("%s: error=%s", msg, err)
			}
			err = fmt.Errorf(msg)
		}
		if err != nil {
			logrus.WithError(err).Error("Final result of CNI DEL was an error.")
		}
	}()

	return
}

func cmdDummyCheck(args *skel.CmdArgs) (err error) {
	fmt.Println("OK")
	return nil
}

func main() {
	Main("0.3.0")
}

func Main(version string) {

	// Use a new flag set so as not to conflict with existing libraries which use "flag"
	flagSet := flag.NewFlagSet("oakestra", flag.ExitOnError)

	// Display the version on "-v"
	versionFlag := flagSet.Bool("v", false, "Display version")

	// Test datastore connection on "-t" this is used to gate installation of the
	// CNI config file, which triggers some orchestrators (K8s included) to start
	// scheduling pods.  By waiting until we get a successful datastore connection
	// test, we can avoid some startup races where host networking to the datastore
	// takes a little while to start up.

	err := flagSet.Parse(os.Args[1:])
	if err != nil {
		cniError := cnitypes.Error{
			Code:    100,
			Msg:     "failed to parse CLI flags",
			Details: err.Error(),
		}
		cniError.Print()
		os.Exit(1)
	}
	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	skel.PluginMain(cmdAdd, cmdDummyCheck, cmdDel,
		cniSpecVersion.PluginSupports("0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0", "1.0.0"),
		"Calico CNI plugin "+version)
}
