package main

import (
  "flag"
  "os"
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

  "github.com/jbergler/node-ip-controller/dns"
)

var settings = struct {
  kubeConfig   string
  masterUrl    string
  project      string
  zone         string
  domain       string
  ttl          int64
}{
  kubeConfig:  "",
  masterUrl:   "",
  project:     os.Getenv("GCP_PROJECT_ID"),
  zone:        os.Getenv("GCP_DNS_ZONE"),
  domain:      os.Getenv("GCP_DNS_DOMAIN"),
  ttl:         60,
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

  var cachedDnsIps []string
  var cachedDnsTs int64

  dnsClient, err := dns.New(settings.project, settings.zone)
  if err != nil {
    klog.Exitf("Error initializing DNS Client: %s", err.Error())
    return
  }

  update := func() {
    klog.V(3).Infoln("Update() running.")
    nodes, err := lister.List(nodeSelector)
    if err != nil {
      klog.Errorf("Error listing Nodes: %s", err.Error())
    }

    var currentLocalIps []string
    for _, node := range nodes {
      if !nodeOk(node) { continue }

      for _, address := range node.Status.Addresses {
        if address.Type == api_core_v1.NodeExternalIP {
          currentLocalIps = append(currentLocalIps, address.Address)
        }
      }
    }

    if cachedDnsTs == 0 || (cachedDnsTs + 300) < time.Now().Unix() {
      record, err := dnsClient.GetRecord(settings.domain, "A")

      if err != nil {
        klog.Errorf("Error getting DNS Record: %s", err.Error())
        return
      }
      if record != nil {
        cachedDnsIps = record.Data
        sort.Strings(cachedDnsIps)
        cachedDnsTs = time.Now().Unix()
        klog.V(0).Infof("Updated DNS cache: %v := %v", cachedDnsTs, cachedDnsIps)
      }
    }

    sort.Strings(currentLocalIps)
    if strings.Join(currentLocalIps, ",") == strings.Join(cachedDnsIps, ",") {
      klog.V(2).Infoln("No changes detected")
      return
    } else {
      klog.V(1).Infof("Detected a change in Node IPs:\n\told: %v\n\tnew: %v", cachedDnsIps, currentLocalIps)
    }

    old_record := &dns.Record{
      Name: settings.domain,
      Type: "A",
      Ttl: settings.ttl,
      Data: cachedDnsIps,
    }
    new_record := &dns.Record{
      Name: settings.domain,
      Type: "A",
      Ttl: settings.ttl,
      Data: currentLocalIps,
    }
    err = dnsClient.ChangeRecord(new_record, old_record)
    if err != nil {
      klog.Errorf("Error updating DNS: %s", err.Error())
    } else {
      cachedDnsTs = 0
      klog.V(1).Infoln("DNS updated successfully")
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
  flag.StringVar(&settings.project, "project", settings.project, "GCP project ID")
  flag.StringVar(&settings.zone, "zone", settings.zone, "GCP DNS zone name")
  flag.StringVar(&settings.domain, "domain", settings.domain, "FQDN to update.")
  klog.InitFlags(nil)
}

func nodeOk(node *api_core_v1.Node) bool {
  for _, condition := range node.Status.Conditions {
    if condition.Type == api_core_v1.NodeReady && condition.Status == api_core_v1.ConditionTrue {
      return true
    }
  }
  return false
}
