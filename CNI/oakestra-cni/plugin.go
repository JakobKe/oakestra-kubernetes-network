package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
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

	logFilePath := "~/oakestra-cni-log.txt"

	logFile, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Fehler beim Öffnen der Logdatei: %v\n", err)
		os.Exit(1)
	}

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

func validatePodname(input string) error {
	regex := `^(\w+)\.(\w+)\.(\w+)\.(\w+)$`
	match, err := regexp.MatchString(regex, input)
	if err != nil {
		return fmt.Errorf("PodName not valid: %w", err)
	}
	if !match {
		return fmt.Errorf("PodName not valid: %w", err)
	}
	return nil
}

func extractServiceNameAndInstanceNumber(input string) (string, int) {
	parts := strings.Split(input, ".")
	serviceName := strings.Join(parts[:len(parts)-1], ".")
	instanceNumber, err := strconv.Atoi(parts[4])
	if err != nil {
		log.Fatalf("error converting the instance number: %v", err)
	}
	return serviceName, instanceNumber
}

func extractPodName(input string) string {
	parts := strings.Split(input, ";")

	for _, part := range parts {
		if strings.HasPrefix(part, "K8S_POD_NAME=") {
			return strings.TrimPrefix(part, "K8S_POD_NAME=")
		}
	}

	return ""
}

func cmdAdd(args *skel.CmdArgs) (err error) {

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

	log.Printf("ADD COMMAND")
	log.Println(args)

	conf := types.NetConf{}
	conf.CNIVersion = "0.3.0"
	var result cniv1.Result

	podName := extractPodName(args.Args)
	if err := validatePodname(podName); err != nil {
		panic(err)
	}
	serviceName, instanceNumber := extractServiceNameAndInstanceNumber(podName)
	parts := strings.Split(args.Netns, "/")
	networkNamespace := parts[len(parts)-1]

	requestBody := connectNetworkRequest{
		NetworkNamespace: networkNamespace,
		Servicename:      serviceName,
		Instancenumber:   instanceNumber,
		PodName:          podName,
	}

	netmanagerURL := "http://localhost:6000/container/deploy"
	_, err = sendDataToNetmanager(requestBody, netmanagerURL)
	if err != nil {
		log.Fatalf("Oakestra NetManager not reachable: %v", err)
	}

	// TODO - This needs to be extended
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

	log.Printf("DEL COMMAND")
	log.Println(args)

	podName := extractPodName(args.Args)
	serviceName, instanceNumber := extractServiceNameAndInstanceNumber(podName)

	requestBody := dettachNetworkRequest{
		Servicename:    serviceName,
		Instancenumber: instanceNumber,
	}

	netmanagerURL := "http://localhost:6000/container/undeploy"
	_, err = sendDataToNetmanager(requestBody, netmanagerURL)
	if err != nil {
		log.Fatalf("Oakestra NetManager not reachable: %v", err)
	}

	return
}

func sendDataToNetmanager(requestBody interface{}, netmanagerURL string) (status string, err error) {
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Printf("Error creating JSON: %v", err)
		return "", err
	}
	resp, err := http.Post(netmanagerURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Printf("Error sending request: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("Server Response: %v", resp.Status)

	return resp.Status, nil
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
		"Oakestra CNI plugin "+version)
}
