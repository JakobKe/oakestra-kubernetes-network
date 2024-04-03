package main

// hier kommen die types rein
// z.b. die Antworten an CNI
// oder die Anfargen an den NodeNetmanager

type connectNetworkRequest struct {
	NetworkNamespace string `json:"networkNamespace"`
	Servicename      string `json:"servicename"`
	Instancenumber   int    `json:"instancenumber"`
	PodName          string `json:"podName"`
	//PortMappings   string `json:"portMappings"` // TODO sollte das hier nicht mehrere Strings vereinbaren?
	// TODO2: Das wird erst später beim NetManager ausgelesen.
}
