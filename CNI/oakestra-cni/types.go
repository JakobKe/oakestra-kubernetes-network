package main

type connectNetworkRequest struct {
	NetworkNamespace string `json:"networkNamespace"`
	Servicename      string `json:"servicename"`
	Instancenumber   int    `json:"instancenumber"`
	PodName          string `json:"podName"`
}

type dettachNetworkRequest struct {
	Servicename    string `json:"serviceName"`
	Instancenumber int    `json:"instanceNumber"`
}
