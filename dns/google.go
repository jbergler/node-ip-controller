package dns

import (
  //"cloud.google.com/go/compute/metadata"
  "time"
  google_dns "google.golang.org/api/dns/v1"

  "golang.org/x/net/context"
  "golang.org/x/oauth2/google"

  //googleapi "google.golang.org/api/googleapi"
)

type Record struct {
	Name string
	Type string
	Ttl  int64
  Data []string
}

type Service struct {
  client  *google_dns.Service
  project string
  zone    string
  timeout time.Duration
}

func New(project string, zone string) (*Service, error) {
  gcloud, err := google.DefaultClient(context.TODO(), google_dns.NdevClouddnsReadwriteScope)
  if err != nil {
    return nil, err
  }

  client, err := google_dns.New(gcloud)
  if err != nil {
    return nil, err
  }

  return &Service {
    client: client,
    project: project,
    zone: zone,
    timeout: 5 * time.Second,
  }, nil
}

func (service *Service) GetRecord(record_name string, record_type string) (*Record, error) {
  var result *google_dns.ResourceRecordSet

  request := service.client.ResourceRecordSets.List(service.project, service.zone)

  err := request.Pages(context.TODO(), func(page *google_dns.ResourceRecordSetsListResponse) error {
    for _, row := range page.Rrsets {
      if row.Name == record_name && row.Type == record_type {
        result = row
      }
    }
    return nil // without this, pagination is broken
  })
  if err != nil {
    return nil, err
  }

  if result == nil {
    return nil, nil
  }

  return &Record {
    Name: result.Name,
    Type: result.Type,
    Ttl:  result.Ttl,
    Data: result.Rrdatas,
  }, nil
}

func (service *Service) ChangeRecord(addition *Record, deletion *Record) error {
  add_rr := []*google_dns.ResourceRecordSet{}
  del_rr := []*google_dns.ResourceRecordSet{}

  if addition != nil && len(addition.Data) > 0 {
    add_rr = append(add_rr, &google_dns.ResourceRecordSet{
	    Name:    addition.Name,
		  Type:    addition.Type,
      Ttl:     addition.Ttl,
      Rrdatas: addition.Data,
    })
  }

  if deletion != nil && len(deletion.Data) > 0 {
		del_rr = append(del_rr, &google_dns.ResourceRecordSet{
			Name:    deletion.Name,
			Type:    deletion.Type,
			Ttl:     deletion.Ttl,
			Rrdatas: deletion.Data,
		})
	}

	change := &google_dns.Change{
		Additions: add_rr,
		Deletions: del_rr,
	}

	_, err := service.client.Changes.Create(service.project, service.zone, change).Context(context.TODO()).Do()
	if err != nil {
		return err
	}

  return nil
}
