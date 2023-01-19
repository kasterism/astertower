package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/kasterism/astertower/pkg/server/executer"
	"github.com/kasterism/astertower/pkg/server/parser"
	"k8s.io/klog/v2"
)

// Receive post requests sent by the front end
type xmlMsg struct {
	XmlStr string `json:"xml"`
}

// Deployment event triggered by the front-end deployment button
func DeployHandler(w http.ResponseWriter, r *http.Request) {
	// Resolve cross-domain problems
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json")

	if r.Method == "POST" {
		klog.Infoln("Handle post request of deployment")

		var xmlmsg xmlMsg
		data, err := io.ReadAll(r.Body)
		if err != nil {
			klog.Errorln(err, "Fail to read request body")
			return
		}
		err = json.Unmarshal(data, &xmlmsg)
		if err != nil {
			klog.Errorln(err, "Fail to parse xml str")
			return
		}

		// Get xml data
		xmlstr := xmlmsg.XmlStr

		// Convert '_' to '-'
		xmlstr = strings.Replace(xmlstr, "_", "-", -1)

		// Parse xml to DAG
		workflowId, dag, err := parser.Xml2Dag(xmlstr)
		if err != nil {
			klog.Errorln(err, "Fail to parse xml")
			return
		}
		// Call executer package
		starter := executer.NewWorkflowStarter(workflowId, dag)
		// Create workflow
		err = starter.CreateWorkflow()
		if err != nil {
			klog.Errorln(err, "Starter create workflow fail")
			return
		}
		_, err = io.WriteString(w, "success")
		if err != nil {
			klog.Errorln(err, "Fail to write response")
			return
		}
	}
}
