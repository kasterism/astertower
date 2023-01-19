package router

import (
	"net/http"

	"github.com/kasterism/astertower/pkg/server/handler"
	"k8s.io/klog/v2"
)

func NewRouter(ip_port string) {

	http.HandleFunc("/deploy", handler.DeployHandler)
	http.HandleFunc("/save", handler.SaveHandler)

	klog.Infoln("Start listen")

	err := http.ListenAndServe(ip_port, nil)
	if err != nil {
		klog.Errorln(err, "Router listen error")
		return
	}
}
