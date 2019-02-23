package main

import (
  "flag"
  "sort"
  "strings"
  "time"

  "k8s.io/client-go/kubernetes"
  _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
  "k8s.io/client-go/tools/clientcmd"
  "k8s.io/klog"
  "k8s.io/apimachinery/pkg/labels"
  "k8s.io/client-go/informers"
  "k8s.io/client-go/tools/cache"
  api_core_v1 "k8s.io/api/core/v1"
)

var settings = struct {
  kubeConfig   string
  masterUrl    string
  domain       string
}{
  kubeConfig:  "",
  masterUrl:   "",
  domain:      "jonasbergler.com",
}

var (
  kubeConfig string
)

func main() {
  flag.Parse()

  config, err := clientcmd.BuildConfigFromFlags(settings.masterUrl, settings.kubeConfig)
  if err != nil {
    klog.Exitf("Error building kubeconfig: %s", err.Error())
  }

  client, err := kubernetes.NewForConfig(config)
  if err != nil {
    klog.Exitf("Error building kubernetes clientset: %s", err.Error())
  }

  stop := make(chan struct{})
  defer close(stop)

  nodeSelector := labels.NewSelector()

  factory := informers.NewSharedInformerFactory(client, time.Minute)
  lister := factory.Core().V1().Nodes().Lister()

  var seenIps []string

  update := func() {
    klog.Infoln("Update() running.")
    nodes, err := lister.List(nodeSelector)
    if err != nil {
      klog.Infoln("failed to list nodes", err)
    }

    var currentIps []string
    for _, node := range nodes {
      if !nodeOk(node) { continue }

      for _, address := range node.Status.Addresses {
        if address.Type == api_core_v1.NodeExternalIP {
          currentIps = append(currentIps, address.Address)
        }
      }
    }

    sort.Strings(currentIps)
    if strings.Join(currentIps, ",") == strings.Join(seenIps, ",") {
      klog.Infoln("no changes detected")
    } else {
      klog.Infof("new ips detected: %v", currentIps)
      seenIps = currentIps
    }
  }

  informer := factory.Core().V1().Nodes().Informer()
  informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
    AddFunc: func(obj interface{}) {
      update()
    },
    UpdateFunc: func(oldObj, newObj interface{}) {
      update()
    },
    DeleteFunc: func(obj interface{}) {
      update()
    },
  })
  informer.Run(stop)

  select {}
}

func init() {
  flag.StringVar(&settings.kubeConfig, "kubeconfig", settings.kubeConfig, "Provide the `path` to a kubeconfig.")
  flag.StringVar(&settings.domain, "domain", settings.domain, "FQDN to update.")
}

func nodeOk(node *api_core_v1.Node) bool {
  for _, condition := range node.Status.Conditions {
    if condition.Type == api_core_v1.NodeReady && condition.Status == api_core_v1.ConditionTrue {
      return true
    }
  }
  return false
}
